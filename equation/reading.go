package equation

import (
	"github.com/theapemachine/datura"
)

/*
Reading exposes one upstream output field as a classifier score source.
*/
type Reading struct {
	field    string
	artifact *datura.Artifact
}

/*
NewReading returns a score source for one output field on the carried artifact.
*/
func NewReading(field string) *Reading {
	return &Reading{
		field:    field,
		artifact: datura.Acquire("equation-reading", datura.APPJSON),
	}
}

func (reading *Reading) Write(p []byte) (int, error) {
	reading.artifact.WithPayload(p)
	return len(p), nil
}

func (reading *Reading) Read(p []byte) (int, error) {
	state, err := stageState(reading.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	value := datura.Peek[float64](state, "output", reading.field)

	return emitOutput(state, p, datura.Map[float64]{"value": value})
}

func (reading *Reading) Close() error {
	return nil
}
