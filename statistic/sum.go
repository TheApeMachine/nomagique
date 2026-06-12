package statistic

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/floats"
)

/*
Sum computes the total of a stream of numbers.
*/
type Sum struct{}

/*
NewSum creates a sum dynamic.
*/
func NewSum() *Sum {
	return &Sum{}
}

/*
Observe returns the sum of the input stream.
*/
func (sum *Sum) Observe(inputs ...core.Number) core.Float64 {
	values := nomagique.Samples(core.Numbers(inputs))

	return core.Float64(floats.Sum(values))
}

func (sum *Sum) Reset() error {
	return nil
}
