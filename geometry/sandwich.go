package geometry

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Sandwich applies a configured motor sandwich to an observed multivector.
Motor components are read from the config artifact "motor" attribute.
*/
type Sandwich struct {
	artifact *datura.Artifact
	motor    Multivector
	output   float64
}

/*
NewSandwich returns a sandwich stage wired from config on the artifact.
*/
func NewSandwich(artifact *datura.Artifact) *Sandwich {
	return &Sandwich{
		artifact: artifact,
	}
}

func (sandwich *Sandwich) Read(payload []byte) (int, error) {
	state := datura.Acquire("sandwich-state", datura.APPJSON)

	if _, err := state.Write(sandwich.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometry-sandwich: state write failed",
			err,
		))
	}

	motorScalars := datura.Peek[[]float64](sandwich.artifact, "motor")

	if len(motorScalars) < multivectorComponentCount {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometry-sandwich: motor requires eight components",
			nil,
		))
	}

	sandwich.motor.FromComponents(motorScalars)

	scalars := datura.Peek[[]float64](state, "batch")

	if len(scalars) < multivectorComponentCount {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometry-sandwich: batch requires target multivector",
			nil,
		))
	}

	var target Multivector

	target.FromComponents(scalars)
	result := sandwich.motor.Sandwich(target)
	sandwich.output = result[MvScalar]
	sandwich.artifact.Poke(sandwich.output, "output", "value")
	state.MergeOutput("value", sandwich.output)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (sandwich *Sandwich) Write(payload []byte) (int, error) {
	sandwich.artifact.WithPayload(payload)
	return len(payload), nil
}

func (sandwich *Sandwich) Close() error {
	return nil
}
