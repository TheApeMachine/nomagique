package logic

/*
Observation carries typed numeric values through logic stages.
*/
type Observation struct {
	Values []float64
}

/*
NewObservation returns a copied typed observation.
*/
func NewObservation(values ...float64) Observation {
	return Observation{
		Values: append([]float64(nil), values...),
	}
}

/*
Condition evaluates whether a circuit rule should fire.
*/
type Condition interface {
	Match(observation Observation) bool
	ResetOperands()
}

/*
ValueSource resolves comparison operands from an observation.
*/
type ValueSource interface {
	Values(observation Observation) []float64
}
