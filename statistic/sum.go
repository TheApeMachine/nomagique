package statistic

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
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

	total := 0.0

	for _, value := range values {
		total += value
	}

	return core.Float64(total)
}

func (sum *Sum) Reset() error {
	return nil
}
