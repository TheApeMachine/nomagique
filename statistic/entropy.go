package statistic

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
Entropy computes normalized Shannon entropy over retained history.
*/
type Entropy struct {
	history []float64
}

/*
NewEntropy returns a typed entropy accumulator.
*/
func NewEntropy() *Entropy {
	return &Entropy{}
}

/*
Measure adds one non-negative sample and returns normalized Shannon entropy.
*/
func (entropy *Entropy) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("entropy", sample); err != nil {
		return ScalarOutput{}, err
	}

	if sample < 0 {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: value must be non-negative",
			nil,
		))
	}

	entropy.history = append(entropy.history, sample)
	total := 0.0

	for _, value := range entropy.history {
		total += value
	}

	output := ScalarOutput{
		Ready: true,
		Count: len(entropy.history),
	}

	if total <= 0 || len(entropy.history) <= 1 {
		return output, nil
	}

	entropySum := 0.0

	for _, value := range entropy.history {
		probability := value / total

		if probability <= 0 {
			continue
		}

		entropySum -= probability * math.Log(probability)
	}

	maxEntropy := math.Log(float64(len(entropy.history)))

	if maxEntropy > 0 {
		output.Value = entropySum / maxEntropy
	}

	return output, nil
}
