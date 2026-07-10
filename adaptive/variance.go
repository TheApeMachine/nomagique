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

	if variance.count == 0 {
		variance.mean = sample
		variance.variance = 0
		variance.prev = sample
		variance.min = sample
		variance.max = sample
		variance.count = 1

		// A single observation has exactly zero variance around itself.
		// That is a defined answer, not a missing one.
		return VarianceOutput{
			Value: 0,
			Mean:  variance.mean,
			Ready: true,
			Count: variance.count,
		}, nil
	}

	variance.min = math.Min(variance.min, sample)
	variance.max = math.Max(variance.max, sample)
	variance.count++
	span := variance.max - variance.min

	if span == 0 {
		variance.prev = sample

		// Every sample observed so far is identical, so the mean is
		// exactly that value even though the variance rate is
		// indeterminate without any observed spread.
		return VarianceOutput{
			Mean:  variance.mean,
			Ready: false,
			Count: variance.count,
		}, nil
	}

	rate := math.Abs(sample-variance.prev) / span
	deviation := sample - variance.mean
	variance.mean += rate * (sample - variance.mean)
	variance.variance += rate * (deviation*deviation - variance.variance)
	variance.prev = sample

	// rate and deviation are both well-defined once span > 0, so the
	// resulting variance is a defined value even when it lands exactly
	// on zero (e.g. the sample landed back on the running mean).
	return VarianceOutput{
		Value: variance.variance,
		Mean:  variance.mean,
		Ready: true,
		Count: variance.count,
	}, nil
}
