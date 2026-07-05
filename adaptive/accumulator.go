package adaptive

/*
Accumulator integrates signed signal strength into a level with no bounds.
*/
type Accumulator struct {
	total float64
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

	accumulator.total += sample
	accumulator.count++

	return AccumulatorOutput{
		Value: accumulator.total,
		Ready: true,
		Count: accumulator.count,
	}, nil
}
