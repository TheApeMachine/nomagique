package equation

import (
	"github.com/theapemachine/datura"
)

/*
Reading exposes one upstream output field as a classifier score source.
*/
type Reading struct {
	artifact *datura.Artifact
	field    string
}

/*
NewReading returns a score source for one output field on the carried artifact.
*/
func NewReading(field string) *Reading {
	return &Reading{
		artifact: datura.Acquire("ladder-reading", datura.APPJSON),
		field:    field,
	}
}

func (reading *Reading) Write(p []byte) (int, error) {
	return reading.artifact.Write(p)
}

func (reading *Reading) Read(p []byte) (int, error) {
	value := datura.Peek[float64](reading.artifact, "output", reading.field)

	reading.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return reading.artifact.Read(p)
}

func (reading *Reading) Close() error {
	return nil
}
