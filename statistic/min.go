package statistic

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/floats"
)

/*
Min computes the smallest value in a stream.
*/
type Min struct{}

/*
NewMin creates a min dynamic.
*/
func NewMin() *Min {
	return &Min{}
}

/*
Observe returns the minimum of the input stream.
*/
func (min *Min) Observe(inputs ...core.Number) core.Float64 {
	values := nomagique.Samples(core.Numbers(inputs))

	if len(values) == 0 {
		return 0
	}

	return core.Float64(floats.Min(values))
}

func (min *Min) Reset() error {
	return nil
}
