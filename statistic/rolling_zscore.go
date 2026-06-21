package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
RollingZScore normalizes the current sample against a rolling return series.
*/
type RollingZScore struct {
	config *datura.Artifact
	bytes  []byte
}

/*
NewRollingZScore returns a rolling z-score stage configured on the artifact.
*/
func NewRollingZScore(config *datura.Artifact) *RollingZScore {
	return &RollingZScore{
		config: config,
	}
}

func (rollingZScore *RollingZScore) Write(payload []byte) (int, error) {
	rollingZScore.bytes = append(rollingZScore.bytes[:0], payload...)

	return len(payload), nil
}

func (rollingZScore *RollingZScore) Read(payload []byte) (int, error) {
	state := datura.Acquire("rolling-zscore-state", datura.APPJSON)

	if _, err := state.Write(rollingZScore.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	returns := datura.Peek[[]float64](rollingZScore.config, "returns")
	score := 0.0

	if len(returns) >= 2 {
		meanReturn := stat.Mean(returns, nil)
		stdReturn := stat.StdDev(returns, nil)

		if stdReturn > 0 {
			score = (sample - meanReturn) / stdReturn
		}
	}

	state.Merge("sample", score)

	return state.Read(payload)
}

func (rollingZScore *RollingZScore) Close() error {
	return nil
}
