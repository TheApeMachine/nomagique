package logic

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/errnie"
)

/*
True always matches when Operand is true, otherwise checks a wired stage.
*/
type True struct {
	Operand bool
	Stage   io.ReadWriteCloser
}

func (trueCondition True) Match(artifact *datura.Artifact) bool {
	if trueCondition.Operand {
		return true
	}

	if trueCondition.Stage == nil {
		return false
	}

	if constant, isConstant := trueCondition.Stage.(*Constant); isConstant {
		return datura.Peek[float64](constant.artifact, "value") != 0
	}

	operand := datura.Acquire("logic-operand", datura.APPJSON)

	packed, err := artifact.Message().MarshalPacked()

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: true condition marshal failed",
			err,
		))
		operand.Release()

		return false
	}

	if _, err = operand.Write(packed); err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: true condition write failed",
			err,
		))
		operand.Release()

		return false
	}

	if err = transport.NewFlipFlop(operand, trueCondition.Stage); err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: true condition flipFlop failed",
			err,
		))
		operand.Release()

		return false
	}

	rootKey := datura.Peek[string](operand, "root")
	inputKeys := datura.Peek[[]string](operand, "inputs")

	if rootKey == "" || len(inputKeys) == 0 {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"logic: true condition wire required",
			nil,
		))
		operand.Release()

		return false
	}

	for _, inputKey := range inputKeys {
		var value float64

		for wireIndex, wireInput := range inputKeys {
			if wireInput != inputKey {
				continue
			}

			if rootKey == "features" {
				features := datura.Peek[[]float64](operand, rootKey)

				if wireIndex >= len(features) {
					errnie.Error(errnie.Err(
						errnie.Validation,
						"logic: true condition feature index out of range",
						nil,
					))
					operand.Release()

					return false
				}

				value = features[wireIndex]
			}

			if rootKey != "features" {
				value = datura.Peek[float64](operand, rootKey, wireInput)
			}

			if value != 0 {
				operand.Release()

				return true
			}
		}
	}

	operand.Release()

	return false
}

func (trueCondition True) ResetOperands() {
	if trueCondition.Stage == nil {
		return
	}

	reset := datura.Acquire("logic-reset", datura.APPJSON)
	reset.Poke(1, "reset")

	packed, err := reset.Message().MarshalPacked()
	reset.Release()

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: true condition reset frame marshal failed",
			err,
		))

		return
	}

	if _, err = trueCondition.Stage.Write(packed); err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: true condition reset stage failed",
			err,
		))
	}
}
