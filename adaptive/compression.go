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

	inputKey := datura.Peek[string](compression.artifact, "compression", "input")
	outputKey := datura.Peek[string](compression.artifact, "compression", "outputKey")

	if inputKey == "" || outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: input and outputKey required",
			nil,
		))
	}

	features := statistic.SnapshotFeatures(state)
	sample, err := statistic.WireScalar(compression.artifact, state, inputKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: sample is non-finite",
			nil,
		))
	}

	baseline := datura.Peek[float64](compression.artifact, "output", "baseline")

	if baseline <= 0 || math.IsNaN(baseline) || math.IsInf(baseline, 0) {
		compression.artifact.Poke(sample, "output", "baseline")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: insufficient samples",
			nil,
		))
	}

	if sample > baseline {
		compression.artifact.Poke(sample, "output", "baseline")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: sample exceeds baseline",
			nil,
		))
	}

	value := (baseline - sample) / baseline

	state.MergeOutput(outputKey, value)
	features.Restore(state)
	state.Merge("root", "output")
	state.Merge("inputs", []string{outputKey})

	return state.Read(payload)
}

func (compression *Compression) Close() error {
	return nil
}
