package adaptive

import "math"

/*
Variance tracks an adaptive mean and variance from the observed sample stream.
*/
type Variance struct {
	mean     float64
	variance float64
	prev     float64
	min      float64
	max      float64
	count    int
}

/*
VarianceOutput reports adaptive variance.
*/
type VarianceOutput struct {
	Value float64
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

	if variance.count == 0 {
		variance.mean = sample
		variance.variance = 0
		variance.prev = sample
		variance.min = sample
		variance.max = sample
		variance.count = 1

		return VarianceOutput{
			Ready: false,
			Count: variance.count,
		}, nil
	}

	variance.min = math.Min(variance.min, sample)
	variance.max = math.Max(variance.max, sample)
	variance.count++
	span := variance.max - variance.min

	if span == 0 {
		variance.prev = sample

		return VarianceOutput{
			Ready: false,
			Count: variance.count,
		}, nil
	}

	rate := math.Abs(sample-variance.prev) / span
	deviation := sample - variance.mean
	variance.mean += rate * (sample - variance.mean)
	variance.variance += rate * (deviation*deviation - variance.variance)
	variance.prev = sample

	return VarianceOutput{
		Value: variance.variance,
		Ready: variance.variance > 0,
		Count: variance.count,
	}, nil
}
