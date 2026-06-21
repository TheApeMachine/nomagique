package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
)

/*
LogReturn computes a lagged log return from a retained sample series on the stage instance.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type LogReturn struct {
	artifact *datura.Artifact
	samples  []float64
}

/*
NewLogReturn returns a log-return stage wired from config attributes on the artifact.
*/
func NewLogReturn(artifact *datura.Artifact) *LogReturn {
	artifact.Inspect("adaptive", "log-return", "NewLogReturn()")

	return &LogReturn{
		artifact: artifact,
		samples:  []float64{},
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

	features := statistic.SnapshotFeatures(state)
	stageKey := logReturn.stageKey()

	if stageKey == "" {
		features.Restore(state)

		return state.Read(payload)
	}

	sample := datura.Peek[float64](state, "sample")

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		features.Restore(state)

		return state.Read(payload)
	}

	returnLag := int(datura.Peek[float64](logReturn.artifact, "inputs", stageKey, "returnLag"))
	longHint := int(datura.Peek[float64](logReturn.artifact, "inputs", stageKey, "longWindow"))

	if returnLag <= 0 {
		returnLag = 1
	}

	samples := logReturn.samples
	samples = append(samples, sample)

	_, longWindow := statistic.RollingWindows(samples, 0, longHint)

	if longWindow > 0 && len(samples) > longWindow+returnLag {
		samples = samples[len(samples)-longWindow-returnLag:]
	}

	logReturn.samples = samples
	logReturnValue := 0.0

	if longWindow > 0 && len(samples) > returnLag {
		anchorSample := samples[len(samples)-returnLag-1]

		if anchorSample > 0 {
			logReturnValue = math.Log(sample / anchorSample)
		}
	}

	state.Merge("sample", logReturnValue)
	features.Restore(state)
	state.Merge("root", "sample")

	return state.Read(payload)
}

func (logReturn *LogReturn) stageKey() string {
	stageKey := datura.Peek[string](logReturn.artifact, "stage")

	if stageKey != "" {
		return stageKey
	}

	order := datura.Peek[[]string](logReturn.artifact, "order")
	stageIndex := int(datura.Peek[float64](logReturn.artifact, "inputs", "precursor", "stageIndex"))

	if stageIndex <= 0 {
		stageIndex = int(datura.Peek[float64](logReturn.artifact, "stageIndex"))
	}

	if stageIndex >= 0 && len(order) > stageIndex {
		return order[stageIndex]
	}

	return ""
}

func (logReturn *LogReturn) Close() error {
	return nil
}
