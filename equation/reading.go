package equation

import (
	"github.com/theapemachine/datura"
)

/*
Reading exposes one upstream output field as a classifier score source.
*/
type Reading struct {
	field string
	bytes []byte
}

/*
NewReading returns a score source for one output field on the carried artifact.
*/
func NewReading(field string) *Reading {
	return &Reading{
		field: field,
	}
}

func (reading *Reading) Write(p []byte) (int, error) {
	reading.bytes = append(reading.bytes[:0], p...)

	return len(p), nil
}

func (reading *Reading) Read(p []byte) (int, error) {
	state, err := stageState(reading.bytes)

	if err != nil {
		return 0, err
	}

	value := datura.Peek[float64](state, "output", reading.field)

	return emitOutput(state, p, datura.Map[float64]{"value": value})
}

func (reading *Reading) Close() error {
	return nil
}
