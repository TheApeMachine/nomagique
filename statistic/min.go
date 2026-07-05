package statistic

/*
Min tracks the smallest streamed sample.
*/
type Min struct {
	value float64
	count int
}

/*
NewMin returns a typed running-minimum accumulator.
*/
func NewMin() *Min {
	return &Min{}
}

/*
Measure adds one sample and returns the running minimum.
*/
func (min *Min) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("min", sample); err != nil {
		return ScalarOutput{}, err
	}

	if min.count == 0 || sample < min.value {
		min.value = sample
	}

	min.count++

	return ScalarOutput{
		Value: min.value,
		Ready: true,
		Count: min.count,
	}, nil
}
