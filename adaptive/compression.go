package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Compression scores how far below the running baseline the current sample sits.
*/
type Compression struct {
	artifact *datura.Artifact
}

/*
NewCompression returns a compression stage ready to bootstrap from its first observation.
*/
func NewCompression(artifact *datura.Artifact) *Compression {
	return &Compression{
		artifact: artifact,
	}
}

func (compression *Compression) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](compression.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return compression.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](compression.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"baseline": sample,
			"value":    0,
		}

		compression.artifact.Poke(output, "output")

		return compression.artifact.Read(p)
	}

	if sample > output["baseline"] {
		output["baseline"] = sample
		output["value"] = 0
		compression.artifact.Poke(output, "output")

		return compression.artifact.Read(p)
	}

	if output["baseline"] == 0 {
		output["value"] = 0
		compression.artifact.Poke(output, "output")

		return compression.artifact.Read(p)
	}

	output["value"] = (output["baseline"] - sample) / output["baseline"]

	compression.artifact.Poke(output, "output")

	return compression.artifact.Read(p)
}

func (compression *Compression) Write(p []byte) (int, error) {
	return compression.artifact.Write(p)
}

func (compression *Compression) Close() error {
	return nil
}
