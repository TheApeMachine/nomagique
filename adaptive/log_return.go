package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
)

/*
LogReturn computes a lagged log return from the retained price ring.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type LogReturn struct {
	artifact *datura.Artifact
}

/*
NewLogReturn returns a log-return stage wired from config attributes on the artifact.
*/
func NewLogReturn(artifact *datura.Artifact) *LogReturn {
	artifact.Inspect("adaptive", "log-return", "NewLogReturn()")

	return &LogReturn{
		artifact: artifact,
	}
}

func (logReturn *LogReturn) Write(payload []byte) (int, error) {
	logReturn.artifact.WithPayload(payload)
	return len(payload), nil
}

func (logReturn *LogReturn) Read(payload []byte) (int, error) {
	state := datura.Acquire("log-return-state", datura.APPJSON)
	state.Inspect("adaptive", "log-return", "Read()", "p")

	if _, err := state.Write(logReturn.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	stageKey := datura.Peek[string](logReturn.artifact, "stage")

	if stageKey == "" {
		order := datura.Peek[[]string](logReturn.artifact, "order")
		stageIndex := int(datura.Peek[float64](logReturn.artifact, "inputs", "precursor", "stageIndex"))

		if stageIndex <= 0 {
			stageIndex = int(datura.Peek[float64](logReturn.artifact, "stageIndex"))
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

	returnLag := int(datura.Peek[float64](logReturn.artifact, "inputs", stageKey, "returnLag"))
	longHint := int(datura.Peek[float64](logReturn.artifact, "inputs", stageKey, "longWindow"))

	if returnLag <= 0 {
		returnLag = 1
	}

	prices := datura.Peek[[]float64](logReturn.artifact, "prices")
	_, longWindow := statistic.RollingWindows(prices, 0, longHint)

	logReturnValue := 0.0

	if longWindow > 0 && len(prices) > returnLag {
		anchorPrice := prices[len(prices)-returnLag-1]

		if anchorPrice > 0 {
			logReturnValue = math.Log(sample / anchorPrice)
			returns := datura.Peek[[]float64](logReturn.artifact, "returns")
			returns = append(returns, logReturnValue)

			if longWindow > 0 && len(returns) > longWindow {
				returns = returns[len(returns)-longWindow:]
			}

			logReturn.artifact.Merge("returns", returns)
		}
	}

	state.Merge("sample", logReturnValue)
	state.Merge("root", "sample")
	state.Merge("inputs", []string{"sample"})
	return state.Read(payload)
}

func (logReturn *LogReturn) Close() error {
	return nil
}
