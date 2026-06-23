package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Compression scores how far below the running baseline the current sample sits.
The constructor artifact holds config; Write buffers inbound payload.
*/
type Compression struct {
	artifact *datura.Artifact
	baseline float64
	ready    bool
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

func (compression *Compression) Write(p []byte) (int, error) {
	compression.artifact.WithPayload(p)
	return len(p), nil
}

func (compression *Compression) Read(payload []byte) (int, error) {
	state := datura.Acquire("compression-state", datura.APPJSON)
	state.Inspect("adaptive", "compression", "Read()", "p")

	if _, err := state.Write(compression.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: state write failed",
			err,
		))
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

	root := datura.Peek[string](state, "root")
	sample := datura.Peek[float64](state, root, inputKey)

	if root == "" {
		sample = datura.Peek[float64](state, inputKey)
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: sample is non-finite",
			nil,
		))
	}

	if !compression.ready || compression.baseline <= 0 {
		compression.baseline = sample
		compression.ready = true

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: insufficient samples",
			nil,
		))
	}

	if sample > compression.baseline {
		compression.baseline = sample

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: sample exceeds baseline",
			nil,
		))
	}

	value := (compression.baseline - sample) / compression.baseline

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (compression *Compression) Close() error {
	return nil
}
