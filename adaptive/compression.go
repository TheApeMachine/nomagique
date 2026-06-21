package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
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

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	baseline := datura.Peek[float64](compression.artifact, "baseline")
	value := 0.0

	switch {
	case baseline == 0 || sample > baseline:
		baseline = sample
	default:
		value = (baseline - sample) / baseline
	}

	compression.artifact.Merge("baseline", baseline)
	state.MergeOutput("baseline", baseline)
	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"baseline", "value"})
	return state.Read(payload)
}

func (compression *Compression) Close() error {
	return nil
}
