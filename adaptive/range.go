package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Range tracks the running span of observed samples.
*/
type Range struct {
	artifact *datura.Artifact
}

/*
NewRange returns a range stage ready to bootstrap from its first observation.
*/
func NewRange() *Range {
	return &Range{
		artifact: datura.Acquire("range", datura.APPJSON).RetainStageAttributes(),
	}
}

func (extent *Range) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](extent.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return extent.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](extent.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"value": 0,
		}

		extent.artifact.Poke(output, "output")

		return extent.artifact.Read(p)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)
	output["value"] = output["max"] - output["min"]

	extent.artifact.Poke(output, "output")

	return extent.artifact.Read(p)
}

func (extent *Range) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](extent.artifact, "output") == nil

	extent.artifact.Clear("sample")

	n, err := extent.artifact.Write(p)

	if bootstrap {
		extent.artifact.Clear("output")
	}

	return n, err
}

func (extent *Range) Close() error {
	return nil
}
