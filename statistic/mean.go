package statistic

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
Mean computes the mean of a stream of numbers.
*/
type Mean struct {
	weights core.Numbers
}

/*
NewMean creates a new mean dynamic.
*/
func NewMean(weights core.Numbers) *Mean {
	return &Mean{
		weights: weights,
	}
}

/*
Observe computes the mean of a stream of numbers.
*/
func (mean *Mean) Observe(inputs ...core.Number) core.Float64 {
	values := nomagique.Samples(core.Numbers(inputs))
	weights := nomagique.Samples(mean.weights)

	if len(weights) == 0 {
		weights = nil
	}

	return core.Float64(stat.Mean(values, weights))
}

func (mean *Mean) Reset() error {
	mean.weights = nil
	return nil
}
