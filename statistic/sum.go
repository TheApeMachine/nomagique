package statistic

/*
Sum integrates streamed samples into a running total.
*/
type Sum struct {
	value float64
	count int
}

/*
NewSum returns a typed running-sum accumulator.
*/
func NewSum() *Sum {
	return &Sum{}
}

/*
Measure adds one sample and returns the running total.
*/
func (sum *Sum) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("sum", sample); err != nil {
		return ScalarOutput{}, err
	}

	sum.value += sample
	sum.count++

	return ScalarOutput{
		Value: sum.value,
		Ready: true,
		Count: sum.count,
	}, nil
}
