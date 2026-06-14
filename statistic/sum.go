package statistic

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/floats"
)

/*
Sum adds every sample in one Observe call.

Plain-language example: three trade sizes (1.2, 0.8, 3.0) sum to 5.0. There is no
memory between calls — each Observe is a fresh total over its inputs.

Sum implements core.Number and fits early in pipelines that later divide by a count
(Mean) or compare against a threshold. Empty input returns zero.
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
