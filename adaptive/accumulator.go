package adaptive

import (
	"github.com/theapemachine/errnie"
)

/*
Accumulator integrates signed signal strength into a level with compensated
summation so long streams retain low-order contributions and reject overflow.
*/
type Accumulator struct {
	total float64
	carry float64
	count int
}

/*
AccumulatorOutput reports the accumulated level.
*/
type AccumulatorOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewAccumulator returns a typed accumulator.
*/
func NewAccumulator() *Accumulator {
	return &Accumulator{}
}

/*
Measure adds one sample and returns the accumulated level.
*/
func (accumulator *Accumulator) Measure(sample float64) (AccumulatorOutput, error) {
	if err := finiteAdaptive("accumulator", sample); err != nil {
		return AccumulatorOutput{}, err
	}

	compensated := sample - accumulator.carry
	next := accumulator.total + compensated
	carry := next - accumulator.total - compensated

	if err := finiteAdaptive("accumulator", next); err != nil {
		return AccumulatorOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"accumulator: sum overflowed to non-finite",
			err,
		))
	}

	accumulator.carry = carry
	accumulator.total = next
	accumulator.count++

	return AccumulatorOutput{
		Value: accumulator.total,
		Ready: true,
		Count: accumulator.count,
	}, nil
}
