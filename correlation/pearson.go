package correlation

import (
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
		artifact: datura.Acquire("pearson", datura.Artifact_Type_json),
		weights:  weights,
	}
}

func (pearson *Pearson) Write(p []byte) (int, error) {
	return pearson.artifact.Write(p)
}

func (pearson *Pearson) Read(p []byte) (int, error) {
	values := float64Batch(pearson.artifact)
	count := len(values)

	if count >= 2 && count%2 == 0 {
		half := count / 2
		left := values[:half]
		right := values[half:]
		putFloat64Payload(&pearson.artifact, "pearson", stat.Correlation(left, right, weightSamples(pearson.weights)))

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
		putFloat64Payload(&pearson.artifact, "pearson", 0)
	}

	return pearson.artifact.Read(p)
}

func (pearson *Pearson) Close() error {
	return nil
}

func (pearson *Pearson) Reset() error {
	pearson.weights = nil

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
