package algorithm

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
ExcitationReading exposes one ExcitationOutcome field as a pipeline score source.
*/
type ExcitationReading struct {
	artifact   *datura.Artifact
	excitation *Excitation
	project    func(ExcitationOutcome) float64
}

func newExcitationReading(
	excitation *Excitation,
	project func(ExcitationOutcome) float64,
) *ExcitationReading {
	return &ExcitationReading{
		artifact:   datura.Acquire("excitation-reading", datura.Artifact_Type_json),
		excitation: excitation,
		project:    project,
	}
}

func (excitation *Excitation) FrenzyReading() *ExcitationReading {
	return newExcitationReading(excitation, func(outcome ExcitationOutcome) float64 {
		return outcome.Frenzy
	})
}

func (excitation *Excitation) SaturationReading() *ExcitationReading {
	return newExcitationReading(excitation, func(outcome ExcitationOutcome) float64 {
		return outcome.Saturation
	})
}

func (excitation *Excitation) OrganicReading() *ExcitationReading {
	return newExcitationReading(excitation, func(outcome ExcitationOutcome) float64 {
		return outcome.Organic
	})
}

func (excitation *Excitation) ExhaustionReading() *ExcitationReading {
	return newExcitationReading(excitation, func(outcome ExcitationOutcome) float64 {
		return outcome.Exhaustion
	})
}

func (reading *ExcitationReading) Write(p []byte) (int, error) {
	reading.artifact.WithPayload(p)
	return len(p), nil
}

func (reading *ExcitationReading) Read(payload []byte) (int, error) {
	state := datura.Acquire("excitation-reading-state", datura.APPJSON)

	if _, err := state.Unpack(reading.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: state write failed",
			err,
		))
	}

	value := 0.0

	if reading.excitation != nil && reading.project != nil {
		value = reading.project(reading.excitation.outcome)
	}

	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (reading *ExcitationReading) Close() error {
	return nil
}
