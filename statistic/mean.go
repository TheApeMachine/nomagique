package statistic

/*
Mean computes a running arithmetic mean of streamed samples.
*/
type Mean struct {
	sum   float64
	count int
}

/*
NewMean returns a typed running-mean accumulator.
*/
func NewMean() *Mean {
	return &Mean{}
}

/*
Measure adds one sample and returns the running arithmetic mean.
*/
func (mean *Mean) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("mean", sample); err != nil {
		return ScalarOutput{}, err
	}

	mean.sum += sample
	mean.count++

	return ScalarOutput{
		Value: mean.sum / float64(mean.count),
		Ready: true,
		Count: mean.count,
	}, nil
}
