package correlation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Pearson computes the Pearson correlation coefficient between two streams.
Optionally, weights can be provided which are applied to the inputs before
computing the correlation. This helps to reduce the impact of outliers.
*/
type Pearson struct {
	artifact *datura.Artifact
	weights  []float64
}

/*
NewPearson creates a new Pearson correlation dynamic.
*/
func NewPearson(weights []float64) *Pearson {
	return &Pearson{
		artifact: datura.Acquire("pearson", datura.APPJSON).RetainStageAttributes(),
		weights:  weights,
	}
}

func (pearson *Pearson) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](pearson.artifact, "output") == nil

	pearson.artifact.Clear("sample")
	pearson.artifact.Clear("paired")
	pearson.artifact.Clear("batch")
	pearson.artifact.Clear("left")
	pearson.artifact.Clear("right")

	n, err := pearson.artifact.Write(p)

	if bootstrap {
		pearson.artifact.Clear("output")
	}

	return n, err
}

func (pearson *Pearson) Read(p []byte) (int, error) {
	values := datura.Peek[[]float64](pearson.artifact, "batch")

	if len(values) == 0 {
		left := datura.Peek[[]float64](pearson.artifact, "left")
		right := datura.Peek[[]float64](pearson.artifact, "right")

		if len(left) > 0 || len(right) > 0 {
			values = append(append([]float64(nil), left...), right...)
		}
	}

	count := len(values)

	if count >= 2 && count%2 == 0 {
		half := count / 2
		left := values[:half]
		right := values[half:]

		weightsOK := len(pearson.weights) == 0 || len(pearson.weights) == half
		weights := pearson.weights

		for _, weight := range weights {
			if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
				weightsOK = false
			}
		}

		if len(weights) == 0 {
			weights = nil
		}

		if !weightsOK {
			pearson.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

			return pearson.artifact.Read(p)
		}

		correlation := stat.Correlation(left, right, weights)

		if math.IsNaN(correlation) || math.IsInf(correlation, 0) {
			correlation = 0
		}

		pearson.artifact.Poke(datura.Map[float64]{"value": correlation}, "output")

		return pearson.artifact.Read(p)
	}

	if count > 0 && count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute Pearson correlation",
			PearsonError(PearsonErrorRequireAtLeastTwoInputs),
		)
	}

	if count%2 != 0 && count > 0 {
		errnie.Err(
			errnie.Validation, "unable to compute Pearson correlation",
			PearsonError(PearsonErrorRequireEqualLength),
		)
	}

	if count == 0 || count < 2 || count%2 != 0 {
		pearson.artifact.Poke(datura.Map[float64]{"value": 0}, "output")
	}

	return pearson.artifact.Read(p)
}

func (pearson *Pearson) Close() error {
	return nil
}

func (pearson *Pearson) Reset() error {
	pearson.weights = nil
	pearson.artifact.Clear("output")

	return nil
}

type PearsonErrorType string

const (
	PearsonErrorRequireAtLeastTwoInputs PearsonErrorType = "require at least two inputs"
	PearsonErrorRequireEqualLength      PearsonErrorType = "require equal length"
)

type PearsonError string

func (pearsonError PearsonError) Error() string {
	return string(pearsonError)
}
