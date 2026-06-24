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
	value    float64
}

/*
NewConstant returns a stage that always emits value.
*/
func NewConstant(value float64) *Constant {
	return &Constant{
		artifact: datura.Acquire("constant", datura.APPJSON),
		value:    value,
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

	state.MergeOutput("value", constant.value)
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
