package equation

import (
	"github.com/theapemachine/errnie"
)

/*
Reading exposes one upstream output field as a classifier score source.
*/
type Reading struct {
	field string
}

/*
NewReading returns a score source for one output field on a typed field map.
*/
func NewReading(field string) *Reading {
	return &Reading{
		field: field,
	}
}

/*
Measure reads the configured field from typed output evidence.
*/
func (reading *Reading) Measure(fields map[string]float64) (float64, error) {
	value, ok := fields[reading.field]

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"reading: field not found",
			nil,
		))
	}

	return value, nil
}
