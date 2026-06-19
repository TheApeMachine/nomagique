package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Accumulator integrates signed signal strength into a level with no bounds.
*/
type Accumulator struct {
	artifact *datura.Artifact
}

/*
NewAccumulator returns an accumulator stage ready for its first observation.
*/
func NewAccumulator() *Accumulator {
	return &Accumulator{
		artifact: datura.Acquire("accumulator", datura.APPJSON),
	}
}

func (accumulator *Accumulator) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](accumulator.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return accumulator.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](accumulator.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"value": 0,
		}
	}

	if sample != 0 {
		output["value"] += sample
	}

	accumulator.artifact.Poke(output, "output")

	return accumulator.artifact.Read(p)
}

func (accumulator *Accumulator) Write(p []byte) (int, error) {
	return accumulator.artifact.Write(p)
}

func (accumulator *Accumulator) Close() error {
	return nil
}
