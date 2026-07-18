package adaptive

import "math"

/*
Variance tracks an adaptive mean and variance from the observed sample stream.
*/
type Variance struct {
	mean  float64
	m2    float64
	count int
}

/*
VarianceOutput reports the adaptive mean and variance.
*/
type VarianceOutput struct {
	Value float64
	Mean  float64
	Ready bool
	Count int
}

/*
NewVariance returns a typed adaptive variance tracker.
*/
func NewVariance() *Variance {
	return &Variance{}
}

/*
Measure adds one sample and returns adaptive variance when ready.
*/
func (variance *Variance) Measure(sample float64) (VarianceOutput, error) {
	if err := finiteAdaptive("variance", sample); err != nil {
		return VarianceOutput{}, err
	}

	variance.count++

	if variance.count == 1 {
		variance.mean = sample
		variance.m2 = 0

		return VarianceOutput{
			Value: 0,
			Mean:  variance.mean,
			Ready: true,
			Count: variance.count,
		}, nil
	}

	delta := sample - variance.mean
	variance.mean += delta / float64(variance.count)
	delta2 := sample - variance.mean
	variance.m2 += delta * delta2

	value := variance.m2 / float64(variance.count-1)

	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}

	return VarianceOutput{
		Value: value,
		Mean:  variance.mean,
		Ready: true,
		Count: variance.count,
	}, nil
}
