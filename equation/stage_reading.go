package equation

import (
	"github.com/theapemachine/datura"
)

/*
StageReading exposes one output field from an artifact stage as a classifier score source.
*/
type StageReading struct {
	artifact *datura.Artifact
	source   ArtifactStage
	field    string
}

/*
NewStageReading returns a score source for one output field on source.
*/
func NewStageReading(source ArtifactStage, field string) *StageReading {
	return &StageReading{
		artifact: datura.Acquire("stage-reading", datura.APPJSON).RetainStageAttributes(),
		source:   source,
		field:    field,
	}
}

func (reading *StageReading) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](reading.artifact, "output") == nil

	reading.artifact.Clear("sample")

	n, err := reading.artifact.Write(p)

	if bootstrap {
		reading.artifact.Clear("output")
	}

	return n, err
}

func (reading *StageReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.source != nil {
		value = datura.Peek[float64](reading.source.StageArtifact(), "output", reading.field)
	}

	reading.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return reading.artifact.Read(p)
}

func (reading *StageReading) Close() error {
	return nil
}
