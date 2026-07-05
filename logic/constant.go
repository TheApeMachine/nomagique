package logic

/*
Constant emits a fixed scalar.
*/
type Constant struct {
	Value float64
}

/*
NewConstant returns a typed constant value source and stage.
*/
func NewConstant(values ...float64) *Constant {
	value := 0.0

	if len(values) > 0 {
		value = values[0]
	}

	return &Constant{
		Value: value,
	}
}

/*
Measure emits the constant as a typed observation.
*/
func (constant *Constant) Measure(observation Observation) (Observation, error) {
	return NewObservation(constant.Value), nil
}

/*
Values returns the constant operand value.
*/
func (constant *Constant) Values(observation Observation) []float64 {
	return []float64{constant.Value}
}
