package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
)

/*
LogReturn computes a lagged log return from the retained price ring.
*/
type LogReturn struct {
	config *datura.Artifact
	bytes  []byte
}

/*
NewLogReturn returns a log-return stage configured on the artifact.
*/
func NewLogReturn(config *datura.Artifact) *LogReturn {
	return &LogReturn{
		config: config,
	}
}

func (logReturn *LogReturn) Write(payload []byte) (int, error) {
	logReturn.bytes = append(logReturn.bytes[:0], payload...)

	return len(payload), nil
}

func (logReturn *LogReturn) Read(payload []byte) (int, error) {
	state := datura.Acquire("log-return-state", datura.APPJSON)

	if _, err := state.Write(logReturn.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	stageKey := datura.Peek[string](logReturn.config, "stage")

	if stageKey == "" {
		order := datura.Peek[[]string](logReturn.config, "order")
		stageIndex := int(datura.Peek[float64](logReturn.config, "inputs", "precursor", "stageIndex"))

		if stageIndex <= 0 {
			stageIndex = int(datura.Peek[float64](logReturn.config, "stageIndex"))
		}

		if stageIndex >= 0 && len(order) > stageIndex {
			stageKey = order[stageIndex]
		}
	}

	if stageKey == "" {
		return state.Read(payload)
	}

	sample := datura.Peek[float64](state, "sample")

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	returnLag := int(datura.Peek[float64](logReturn.config, "inputs", stageKey, "returnLag"))
	longHint := int(datura.Peek[float64](logReturn.config, "inputs", stageKey, "longWindow"))

	if returnLag <= 0 {
		returnLag = 1
	}

	prices := datura.Peek[[]float64](logReturn.config, "prices")
	_, longWindow := statistic.RollingWindows(prices, 0, longHint)

	logReturnValue := 0.0

	if longWindow > 0 && len(prices) > returnLag {
		anchorPrice := prices[len(prices)-returnLag-1]

		if anchorPrice > 0 {
			logReturnValue = math.Log(sample / anchorPrice)
			returns := datura.Peek[[]float64](logReturn.config, "returns")
			returns = append(returns, logReturnValue)

			if longWindow > 0 && len(returns) > longWindow {
				returns = returns[len(returns)-longWindow:]
			}

			logReturn.config.Merge("returns", returns)
		}
	}

	state.Merge("sample", logReturnValue)

	return state.Read(payload)
}

func (logReturn *LogReturn) Close() error {
	return nil
}
