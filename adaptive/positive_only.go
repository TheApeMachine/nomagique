package adaptive

import "math"

/*
PositiveOnly optionally clamps a sample at zero from below.
*/
type PositiveOnly struct {
	enabled bool
	count   int
}

/*
PositiveOnlyOutput reports the non-negative transformed sample.
*/
type PositiveOnlyOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewPositiveOnly returns a typed positive-only gate.
*/
func NewPositiveOnly(enabled ...bool) *PositiveOnly {
	positiveOnly := &PositiveOnly{}

	if len(enabled) > 0 {
		positiveOnly.enabled = enabled[0]
	}

	return positiveOnly
}

/*
Measure returns the sample or zero when the gate is enabled and the sample is negative.
*/
func (positiveOnly *PositiveOnly) Measure(sample float64) (PositiveOnlyOutput, error) {
	if err := finiteAdaptive("positive-only", sample); err != nil {
		return PositiveOnlyOutput{}, err
	}

	if positiveOnly.enabled {
		sample = math.Max(0, sample)
	}

	positiveOnly.count++

	return PositiveOnlyOutput{
		Value: sample,
		Ready: true,
		Count: positiveOnly.count,
	}, nil
}
