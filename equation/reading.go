package equation

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/causal"
)

/*
Reading exposes one ladder output field as a pipeline score source.
*/
type Reading struct {
	artifact *datura.Artifact
	source   *causal.Ladder
	field    string
}

/*
NewReading returns a score source for one ladder output field.
*/
func NewReading(source *causal.Ladder, field string) *Reading {
	return &Reading{
		artifact: datura.Acquire("ladder-reading", datura.APPJSON).RetainStageAttributes(),
		source:   source,
		field:    field,
	}
}

func (reading *Reading) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](reading.artifact, "output") == nil

	reading.artifact.Clear("sample")

	n, err := reading.artifact.Write(p)

	if bootstrap {
		reading.artifact.Clear("output")
	}

	return n, err
}

func (reading *Reading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.source != nil {
		value = datura.Peek[float64](reading.source.Artifact(), "output", reading.field)
	}

	reading.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return reading.artifact.Read(p)
}

func (reading *Reading) Close() error {
	return nil
}
