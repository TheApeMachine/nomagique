package statistic

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Sum integrates streamed samples into a running total.
*/
type Sum struct {
	artifact *datura.Artifact
}

/*
NewSum creates a sum stage.
*/
func NewSum() *Sum {
	return &Sum{
		artifact: datura.Acquire("sum", datura.APPJSON).RetainStageAttributes(),
	}
}

func (sum *Sum) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](sum.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return sum.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](sum.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"value": 0,
		}
	}

	output["value"] += sample
	sum.artifact.Poke(output, "output")

	return sum.artifact.Read(p)
}

func (sum *Sum) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](sum.artifact, "output") == nil

	sum.artifact.Clear("sample")

	n, err := sum.artifact.Write(p)

	if bootstrap {
		sum.artifact.Clear("output")
	}

	return n, err
}

func (sum *Sum) Close() error {
	return nil
}
