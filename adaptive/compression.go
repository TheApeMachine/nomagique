package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Compression scores how far below the running baseline the current sample sits.
The constructor artifact holds config; Write buffers inbound payload.
*/
type Compression struct {
	artifact  *datura.Artifact
	baselines map[string]float64
}

/*
NewCompression returns a compression stage wired from config attributes on the artifact.
*/
func NewCompression(artifact *datura.Artifact) *Compression {
	return &Compression{
		artifact:  artifact,
		baselines: map[string]float64{},
	}
}

func (compression *Compression) Write(p []byte) (int, error) {
	compression.artifact.WithPayload(p)
	return len(p), nil
}

func (compression *Compression) Read(payload []byte) (int, error) {
	state := datura.Acquire("compression-state", datura.APPJSON)

	if _, err := state.Write(compression.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: state write failed",
			err,
		))
	}

	state.Inspect("adaptive", "compression", "Read()", "p")

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

	seriesKey := statistic.SeriesKey(compression.artifact, state, "compression")
	baseline := compression.baselines[seriesKey]

	if baseline <= 0 {
		compression.baselines[seriesKey] = sample
		state.MergeOutput(outputKey, 0.0)
		state.Poke("output", "root")
		state.Poke([]string{outputKey}, "inputs")

		return state.Read(payload)
	}

	if sample > baseline {
		compression.baselines[seriesKey] = sample
		state.MergeOutput(outputKey, 0.0)
		state.Poke("output", "root")
		state.Poke([]string{outputKey}, "inputs")

		return state.Read(payload)
	}

	value := (baseline - sample) / baseline

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (compression *Compression) Close() error {
	return nil
}
