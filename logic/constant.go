package logic

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Constant emits a fixed scalar on every Read.
*/
type Constant struct {
	artifact *datura.Artifact
}

/*
NewConstant returns a stage that always emits the artifact "value" attribute.
*/
func NewConstant(artifact *datura.Artifact) *Constant {
	return &Constant{
		artifact: artifact,
	}
}

func (constant *Constant) Read(payload []byte) (int, error) {
	state := datura.Acquire("constant-state", datura.APPJSON)

	if _, err := state.Write(constant.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.IO,
			"logic: constant state write failed",
			err,
		))
	}

	value := datura.Peek[float64](constant.artifact, "value")
	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (constant *Constant) Write(payload []byte) (int, error) {
	constant.artifact.WithPayload(payload)
	return len(payload), nil
}

func (constant *Constant) Close() error {
	return nil
}
