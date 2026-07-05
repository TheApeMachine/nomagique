package adaptive

import "math"

/*
Range tracks the running span of observed samples.
*/
type Range struct {
	min   float64
	max   float64
	count int
}

/*
RangeOutput reports the observed sample range.
*/
type RangeOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewRange returns a typed range tracker.
*/
func NewRange() *Range {
	return &Range{}
}

/*
Measure adds one sample and returns the running span when non-zero.
*/
func (extent *Range) Measure(sample float64) (RangeOutput, error) {
	if err := finiteAdaptive("range", sample); err != nil {
		return RangeOutput{}, err
	}

	if extent.count == 0 {
		extent.min = sample
		extent.max = sample
		extent.count = 1

		return RangeOutput{
			Ready: false,
			Count: extent.count,
		}, nil
	}

	extent.min = math.Min(extent.min, sample)
	extent.max = math.Max(extent.max, sample)
	extent.count++
	span := extent.max - extent.min

	return RangeOutput{
		Value: span,
		Ready: span > 0,
		Count: extent.count,
	}, nil
}
