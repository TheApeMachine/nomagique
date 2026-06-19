package equation

import (
	"github.com/theapemachine/datura"
)

/*
StageReading exposes one output field from the carried artifact as a classifier score source.
*/
type StageReading struct {
	artifact *datura.Artifact
	field    string
}

/*
NewStageReading returns a score source for one output field on the carried artifact.
*/
func NewStageReading(field string) *StageReading {
	return &StageReading{
		artifact: datura.Acquire("stage-reading", datura.APPJSON),
		field:    field,
	}
}

func (reading *StageReading) Write(p []byte) (int, error) {
	return reading.artifact.Write(p)
}

func (reading *StageReading) Read(p []byte) (int, error) {
	value := datura.Peek[float64](reading.artifact, "output", reading.field)

	reading.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return reading.artifact.Read(p)
}

func (reading *StageReading) Close() error {
	return nil
}
