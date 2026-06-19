package statistic

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Mean computes a running arithmetic mean of streamed samples.
*/
type Mean struct {
	artifact *datura.Artifact
}

/*
NewMean creates a mean stage.
*/
func NewMean() *Mean {
	return &Mean{
		artifact: datura.Acquire("mean", datura.APPJSON).RetainStageAttributes(),
	}
}

func (mean *Mean) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](mean.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return mean.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](mean.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"count": 0,
			"sum":   0,
			"value": 0,
		}
	}

	output["count"]++
	output["sum"] += sample

	if output["count"] > 0 {
		output["value"] = output["sum"] / output["count"]
	}

	mean.artifact.Poke(output, "output")

	return mean.artifact.Read(p)
}

func (mean *Mean) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](mean.artifact, "output") == nil

	mean.artifact.Clear("sample")

	n, err := mean.artifact.Write(p)

	if bootstrap {
		mean.artifact.Clear("output")
	}

	return n, err
}

func (mean *Mean) Close() error {
	return nil
}
