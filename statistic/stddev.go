package statistic

import "gonum.org/v1/gonum/stat"

/*
StdDev computes the sample standard deviation over retained history.
*/
type StdDev struct {
	history []float64
}

/*
NewStdDev returns a typed standard-deviation accumulator.
*/
func NewStdDev() *StdDev {
	return &StdDev{}
}

/*
Measure adds one sample and returns sample standard deviation when ready.
*/
func (stdDev *StdDev) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("stddev", sample); err != nil {
		return ScalarOutput{}, err
	}

	stdDev.history = append(stdDev.history, sample)

	if len(stdDev.history) < 2 {
		return ScalarOutput{
			Ready: false,
			Count: len(stdDev.history),
		}, nil
	}

	return ScalarOutput{
		Value: stat.StdDev(stdDev.history, nil),
		Ready: true,
		Count: len(stdDev.history),
	}, nil
}
