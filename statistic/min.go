package statistic

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Min tracks the smallest streamed sample.
*/
type Min struct {
	artifact *datura.Artifact
}

/*
NewMin creates a min stage.
*/
func NewMin() *Min {
	return &Min{
		artifact: datura.Acquire("min", datura.APPJSON).RetainStageAttributes(),
	}
}

func (min *Min) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](min.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return min.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](min.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"value": sample,
		}

		min.artifact.Poke(output, "output")

		return min.artifact.Read(p)
	}

	if sample < output["value"] {
		output["value"] = sample
	}

	min.artifact.Poke(output, "output")

	return min.artifact.Read(p)
}

func (min *Min) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](min.artifact, "output") == nil

	min.artifact.Clear("sample")

	n, err := min.artifact.Write(p)

	if bootstrap {
		min.artifact.Clear("output")
	}

	return n, err
}

func (min *Min) Close() error {
	return nil
}
