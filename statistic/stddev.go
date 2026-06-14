package statistic

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
StdDev computes the sample standard deviation of a stream of numbers.

Trading example: pass session returns for one symbol (or residuals from a
forecast stage) to measure realized volatility. Optional weights turn the
stream into a weighted dispersion when some observations carry more evidence
than others.
*/
type StdDev struct {
	weights core.Numbers
}

/*
NewStdDev creates a standard-deviation dynamic.
*/
func NewStdDev(weights core.Numbers) *StdDev {
	return &StdDev{
		weights: weights,
	}
}

/*
Observe returns the sample standard deviation of the input stream.
*/
func (stdDev *StdDev) Observe(inputs ...core.Number) core.Float64 {
	values := nomagique.Samples(core.Numbers(inputs))
	weights := nomagique.Samples(stdDev.weights)

	if len(weights) == 0 {
		weights = nil
	}

	return core.Float64(stat.StdDev(values, weights))
}

func (stdDev *StdDev) Reset() error {
	stdDev.weights = nil
	return nil
}
