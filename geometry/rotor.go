package geometry

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Rotor builds a rotation motor from angle and axis bivector scalars.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Rotor struct {
	artifact    *datura.Artifact
	multivector Multivector
	output      float64
}

/*
NewRotor returns a rotor stage wired from config attributes on the artifact.
*/
func NewRotor(artifact *datura.Artifact) *Rotor {
	return &Rotor{
		artifact: artifact,
	}
}

func (rotor *Rotor) Read(payload []byte) (int, error) {
	state := datura.Acquire("rotor-state", datura.APPJSON)

	if _, err := state.Unpack(rotor.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometry-rotor: state write failed",
			err,
		))
	}

	scalars := datura.Peek[[]float64](state, "batch")

	if len(scalars) < 4 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometry-rotor: batch requires angle and axis",
			nil,
		))
	}

	rotor.multivector.FromRotation(scalars[0], scalars[1], scalars[2], scalars[3])
	rotor.output = rotor.multivector[MvScalar]
	rotor.artifact.Poke(rotor.output, "output", "value")
	rotor.artifact.Poke(multivectorSlice(rotor.multivector), "output", "motor")
	state.MergeOutput("value", rotor.output)
	state.MergeOutput("motor", multivectorSlice(rotor.multivector))
	state.Poke("output", "root")
	state.Poke([]string{"value", "motor"}, "inputs")

	return state.PackInto(payload)
}

func (rotor *Rotor) Write(payload []byte) (int, error) {
	rotor.artifact.WithPayload(payload)
	return len(payload), nil
}

func (rotor *Rotor) Close() error {
	return nil
}
