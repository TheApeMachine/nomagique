package statistic

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Max tracks the largest streamed sample.
*/
type Max struct {
	artifact *datura.Artifact
}

/*
NewMax creates a max stage.
*/
func NewMax() *Max {
	return &Max{
		artifact: datura.Acquire("max", datura.APPJSON),
	}
}

func (max *Max) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](max.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return max.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](max.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"value": sample,
		}

		max.artifact.Poke(output, "output")

		return max.artifact.Read(p)
	}

	if sample > output["value"] {
		output["value"] = sample
	}

	max.artifact.Poke(output, "output")

	return max.artifact.Read(p)
}

func (max *Max) Write(p []byte) (int, error) {
	return max.artifact.Write(p)
}

func (max *Max) Close() error {
	return nil
}
