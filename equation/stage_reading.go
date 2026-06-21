package equation

import (
	"github.com/theapemachine/datura"
)

/*
StageReading exposes one output field from the carried artifact as a classifier score source.
*/
type StageReading struct {
	field string
	bytes []byte
}

/*
NewStageReading returns a score source for one output field on the carried artifact.
*/
func NewStageReading(field string) *StageReading {
	return &StageReading{
		field: field,
	}
}

func (reading *StageReading) Write(p []byte) (int, error) {
	reading.bytes = append(reading.bytes[:0], p...)

	return len(p), nil
}

func (reading *StageReading) Read(p []byte) (int, error) {
	state, err := stageState(reading.bytes)

	if err != nil {
		return 0, err
	}

	value := datura.Peek[float64](state, "output", reading.field)

	return emitOutput(state, p, datura.Map[float64]{"value": value})
}

func (reading *StageReading) Close() error {
	return nil
}
