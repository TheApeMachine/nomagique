package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Compression scores how far below the running baseline the current sample sits.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Compression struct {
	artifact *datura.Artifact
	baseline float64
}

/*
NewCompression returns a compression stage wired from config attributes on the artifact.
*/
func NewCompression(artifact *datura.Artifact) *Compression {
	artifact.Inspect("adaptive", "compression", "NewCompression()")

	return &Compression{
		artifact: artifact,
	}
}

func (compression *Compression) Write(payload []byte) (int, error) {
	compression.artifact.WithPayload(payload)
	return len(payload), nil
}

func (compression *Compression) Read(payload []byte) (int, error) {
	state := datura.Acquire("compression-state", datura.APPJSON)
	state.Inspect("adaptive", "compression", "Read()", "p")

	if _, err := state.Write(compression.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := statistic.SnapshotFeatures(state)
	sample := compression.sample(state)

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: sample is non-finite",
			nil,
		))
	}

	baseline := compression.baseline
	value := 0.0

	switch {
	case baseline <= 0 || sample > baseline:
		baseline = sample
	default:
		value = (baseline - sample) / baseline
	}

	compression.baseline = baseline
	outputKey := datura.Peek[string](compression.artifact, "compression", "outputKey")

	if outputKey == "" {
		outputKey = "value"
	}

	state.MergeOutput(outputKey, value)
	features.Restore(state)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"baseline", outputKey})

	return state.Read(payload)
}

func (compression *Compression) Close() error {
	return nil
}

func (compression *Compression) sample(state *datura.Artifact) float64 {
	inputKey := datura.Peek[string](compression.artifact, "compression", "input")

	if inputKey == "" {
		inputKey = "sample"
	}

	body := datura.As[datura.Map[any]](state)

	if body != nil {
		output, ok := body["output"].(map[string]any)

		if ok {
			if _, present := output[inputKey]; present {
				return datura.Peek[float64](state, "output", inputKey)
			}
		}
	}

	return datura.Peek[float64](state, inputKey)
}
