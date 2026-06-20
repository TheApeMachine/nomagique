package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Compression scores how far below the running baseline the current sample sits.
The config artifact holds the rolling baseline across frames; the inbound
artifact wire is buffered between Write and Read.
*/
type Compression struct {
	config *datura.Artifact
	bytes  []byte
}

/*
NewCompression returns a compression stage ready to bootstrap from its first observation.
*/
func NewCompression(config *datura.Artifact) *Compression {
	return &Compression{
		config: config,
	}
}

func (compression *Compression) Write(p []byte) (int, error) {
	compression.bytes = append(compression.bytes[:0], p...)

	return len(p), nil
}

func (compression *Compression) Read(p []byte) (int, error) {
	state := datura.Acquire("compression-state", datura.APPJSON)

	if _, err := state.Write(compression.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(p)
	}

	baseline := datura.Peek[float64](compression.config, "baseline")
	value := 0.0

	switch {
	case baseline == 0 || sample > baseline:
		baseline = sample
	default:
		value = (baseline - sample) / baseline
	}

	compression.config.Merge("baseline", baseline)

	output := datura.Acquire("compression-output", datura.APPJSON)
	output.WithPayload(state.DecryptPayload())
	output.MergeOutput("baseline", baseline)
	output.MergeOutput("value", value)

	return output.Read(p)
}

func (compression *Compression) Close() error {
	return nil
}
