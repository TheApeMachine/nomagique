package logic

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/errnie"
)

/*
LessThan compares input samples against a wired operand stage.
*/
type LessThan struct {
	Right io.ReadWriteCloser
}

func (lessThan LessThan) Match(artifact *datura.Artifact) bool {
	rootKey := datura.Peek[string](artifact, "root")
	inputKeys := datura.Peek[[]string](artifact, "inputs")

	if rootKey == "" || len(inputKeys) == 0 {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"logic: lessThan wire required",
			nil,
		))

		return false
	}

	samples := make([]float64, len(inputKeys))

	for index, inputKey := range inputKeys {
		var sample float64

		for wireIndex, wireInput := range inputKeys {
			if wireInput != inputKey {
				continue
			}

			if rootKey == "features" {
				features := datura.Peek[[]float64](artifact, rootKey)

				if wireIndex >= len(features) {
					errnie.Error(errnie.Err(
						errnie.Validation,
						"logic: greaterThan feature index out of range",
						nil,
					))

					return false
				}

				sample = features[wireIndex]
			}

			if rootKey != "features" {
				sample = datura.Peek[float64](artifact, rootKey, wireInput)
			}
		}

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"logic: lessThan input sample is non-finite",
				nil,
			))

			return false
		}

		samples[index] = sample
	}

	var rightScalars []float64

	if constant, isConstant := lessThan.Right.(*Constant); isConstant {
		rightScalars = []float64{datura.Peek[float64](constant.artifact, "value")}
	} else {
		rightScratch := datura.Acquire("logic-operand-right", datura.APPJSON)

		packed, err := artifact.Message().MarshalPacked()

		if err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: lessThan operand marshal failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		if _, err = rightScratch.Write(packed); err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: lessThan operand write failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		if err = transport.NewFlipFlop(rightScratch, lessThan.Right); err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: lessThan operand flipFlop failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		rightKeys := datura.Peek[[]string](rightScratch, "inputs")

		if len(rightKeys) == 0 {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"logic: lessThan operand wire required",
				nil,
			))
			rightScratch.Release()

			return false
		}

		rightRoot := datura.Peek[string](rightScratch, "root")

		if rightRoot == "" {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"logic: greaterThan operand root required",
				nil,
			))
			rightScratch.Release()

			return false
		}

		rightScalars = make([]float64, len(rightKeys))

		for index, rightKey := range rightKeys {
			var right float64

			for wireIndex, wireInput := range rightKeys {
				if wireInput != rightKey {
					continue
				}

				if rightRoot == "features" {
					features := datura.Peek[[]float64](rightScratch, rightRoot)

					if wireIndex >= len(features) {
						errnie.Error(errnie.Err(
							errnie.Validation,
							"logic: greaterThan operand feature index out of range",
							nil,
						))
						rightScratch.Release()

						return false
					}

					right = features[wireIndex]
				}

				if rightRoot != "features" {
					right = datura.Peek[float64](rightScratch, rightRoot, wireInput)
				}
			}

			rightScalars[index] = right
		}

		rightScratch.Release()
	}

	for _, sample := range samples {
		for _, right := range rightScalars {
			if sample >= right {
				return false
			}
		}
	}

	return true
}

func (lessThan LessThan) ResetOperands() {
	if lessThan.Right == nil {
		return
	}

	reset := datura.Acquire("logic-reset", datura.APPJSON)
	reset.Poke(1, "reset")

	packed, err := reset.Message().MarshalPacked()
	reset.Release()

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: lessThan reset frame marshal failed",
			err,
		))

		return
	}

	if _, err = lessThan.Right.Write(packed); err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: lessThan reset stage failed",
			err,
		))
	}
}
