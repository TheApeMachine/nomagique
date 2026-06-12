package statistic

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

/*
Max computes the largest value in a stream.
*/
type Max struct{}

/*
NewMax creates a max dynamic.
*/
func NewMax() *Max {
	return &Max{}
}

/*
Observe returns the maximum of the input stream.
*/
func (max *Max) Observe(inputs ...core.Number) core.Float64 {
	values := nomagique.Samples(core.Numbers(inputs))

	if len(values) == 0 {
		return 0
	}

	maxValue := values[0]

	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}

	return core.Float64(maxValue)
}

func (max *Max) Reset() error {
	return nil
}
