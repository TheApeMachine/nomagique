package equation

import (
	"github.com/theapemachine/datura"
)

/*
StageReading exposes one output field from the carried artifact as a classifier score source.
*/
type StageReading struct {
	field string
	artifact *datura.Artifact
}

/*
NewStageReading returns a score source for one output field on the carried artifact.
*/
func NewStageReading(field string) *StageReading {
	return &StageReading{
		field: field,
		artifact: datura.Acquire("equation-stage-reading", datura.APPJSON),
	}
}

func (reading *StageReading) Write(p []byte) (int, error) {
	reading.artifact.WithPayload(p)
	return len(p), nil
}

func (reading *StageReading) Read(p []byte) (int, error) {
	state, err := stageState(reading.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	value := datura.Peek[float64](state, "output", reading.field)

	return emitOutput(state, p, datura.Map[float64]{"value": value})
}

func (reading *StageReading) Close() error {
	return nil
}
