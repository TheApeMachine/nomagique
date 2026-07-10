package adaptive

import "math"

/*
Delta tracks a unit-normalized absolute change relative to the running sample range.
*/
type Delta struct {
	min   float64
	max   float64
	prev  float64
	count int
}

/*
DeltaOutput reports normalized absolute change.
*/
type DeltaOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewDelta returns a typed delta tracker.
*/
func NewDelta() *Delta {
	return &Delta{}
}

/*
Measure adds one sample and returns absolute change normalized by running range.
*/
func (delta *Delta) Measure(sample float64) (DeltaOutput, error) {
	if err := finiteAdaptive("delta", sample); err != nil {
		return DeltaOutput{}, err
	}

	if delta.count == 0 {
		delta.min = sample
		delta.max = sample
		delta.prev = sample
		delta.count = 1

		// No prior observation exists, so the observed range before this
		// sample was empty. This single reading defines the entirety of
		// that range, making the normalized change maximal by definition.
		return DeltaOutput{
			Value: 1,
			Ready: true,
			Count: delta.count,
		}, nil
	}

	delta.min = math.Min(delta.min, sample)
	delta.max = math.Max(delta.max, sample)
	span := delta.max - delta.min

	if span == 0 {
		delta.prev = sample
		delta.count++

		return DeltaOutput{
			Ready: false,
			Count: delta.count,
		}, nil
	}

	value := math.Abs(sample-delta.prev) / span
	delta.prev = sample
	delta.count++

	return DeltaOutput{
		Value: value,
		Ready: true,
		Count: delta.count,
	}, nil
}
