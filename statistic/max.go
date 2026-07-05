package statistic

/*
Max tracks the largest streamed sample.
*/
type Max struct {
	value float64
	count int
}

/*
NewMax returns a typed running-maximum accumulator.
*/
func NewMax() *Max {
	return &Max{}
}

/*
Measure adds one sample and returns the running maximum.
*/
func (max *Max) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("max", sample); err != nil {
		return ScalarOutput{}, err
	}

	if max.count == 0 || sample > max.value {
		max.value = sample
	}

	max.count++

	return ScalarOutput{
		Value: max.value,
		Ready: true,
		Count: max.count,
	}, nil
}
