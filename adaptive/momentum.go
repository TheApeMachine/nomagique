package adaptive

import "math"

/*
Momentum tracks a signed unit-normalized move relative to the running range.
*/
type Momentum struct {
	min   float64
	max   float64
	prev  float64
	count int
}

/*
MomentumOutput reports normalized signed change.
*/
type MomentumOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewMomentum returns a typed momentum tracker.
*/
func NewMomentum() *Momentum {
	return &Momentum{}
}

/*
Measure adds one sample and returns signed change normalized by running range.
*/
func (momentum *Momentum) Measure(sample float64) (MomentumOutput, error) {
	if err := finiteAdaptive("momentum", sample); err != nil {
		return MomentumOutput{}, err
	}

	if momentum.count == 0 {
		momentum.min = sample
		momentum.max = sample
		momentum.prev = sample
		momentum.count = 1

		return MomentumOutput{
			Ready: false,
			Count: momentum.count,
		}, nil
	}

	momentum.min = math.Min(momentum.min, sample)
	momentum.max = math.Max(momentum.max, sample)
	span := momentum.max - momentum.min

	if span == 0 {
		momentum.prev = sample
		momentum.count++

		return MomentumOutput{
			Ready: false,
			Count: momentum.count,
		}, nil
	}

	value := (sample - momentum.prev) / span
	momentum.prev = sample
	momentum.count++

	return MomentumOutput{
		Value: value,
		Ready: true,
		Count: momentum.count,
	}, nil
}
