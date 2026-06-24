package logic

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/errnie"
)

/*
Condition evaluates whether a circuit rule should fire.
*/
type Condition interface {
	Match(artifact *datura.Artifact) bool
	ResetOperands()
}

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
		return constant.value != 0
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

/*
And matches when every nested condition matches.
*/
type And []Condition

func (andCondition And) Match(artifact *datura.Artifact) bool {
	for _, operand := range andCondition {
		if !operand.Match(artifact) {
			return false
		}
	}

	return len(andCondition) > 0
}

func (andCondition And) ResetOperands() {
	for _, operand := range andCondition {
		operand.ResetOperands()
	}
}

/*
Or matches when any nested condition matches.
*/
type Or []Condition

func (orCondition Or) Match(artifact *datura.Artifact) bool {
	for _, operand := range orCondition {
		if operand.Match(artifact) {
			return true
		}
	}

	return false
}

func (orCondition Or) ResetOperands() {
	for _, operand := range orCondition {
		operand.ResetOperands()
	}
}

/*
Not inverts a nested condition.
*/
type Not struct {
	Operand Condition
}

func (notCondition Not) Match(artifact *datura.Artifact) bool {
	return !notCondition.Operand.Match(artifact)
}

func (notCondition Not) ResetOperands() {
	notCondition.Operand.ResetOperands()
}

/*
Xor matches when exactly one nested condition matches.
*/
type Xor []Condition

func (xorCondition Xor) Match(artifact *datura.Artifact) bool {
	matches := 0

	for _, operand := range xorCondition {
		if operand.Match(artifact) {
			matches++
		}
	}

	return matches == 1
}

func (xorCondition Xor) ResetOperands() {
	for _, operand := range xorCondition {
		operand.ResetOperands()
	}
}

/*
GreaterThan compares input samples against a wired operand stage.
*/
type GreaterThan struct {
	Right io.ReadWriteCloser
}

func (greaterThan GreaterThan) Match(artifact *datura.Artifact) bool {
	rootKey := datura.Peek[string](artifact, "root")
	inputKeys := datura.Peek[[]string](artifact, "inputs")

	if rootKey == "" || len(inputKeys) == 0 {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"logic: greaterThan wire required",
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
				"logic: greaterThan input sample is non-finite",
				nil,
			))

			return false
		}

		samples[index] = sample
	}

	var rightScalars []float64

	if constant, isConstant := greaterThan.Right.(*Constant); isConstant {
		rightScalars = []float64{constant.value}
	} else {
		rightScratch := datura.Acquire("logic-operand-right", datura.APPJSON)

		packed, err := artifact.Message().MarshalPacked()

		if err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: greaterThan operand marshal failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		if _, err = rightScratch.Write(packed); err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: greaterThan operand write failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		if err = transport.NewFlipFlop(rightScratch, greaterThan.Right); err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: greaterThan operand flipFlop failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		rightKeys := datura.Peek[[]string](rightScratch, "inputs")

		if len(rightKeys) == 0 {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"logic: greaterThan operand wire required",
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
			if sample <= right {
				return false
			}
		}
	}

	return true
}

func (greaterThan GreaterThan) ResetOperands() {
	if greaterThan.Right == nil {
		return
	}

	reset := datura.Acquire("logic-reset", datura.APPJSON)
	reset.Poke(1, "reset")

	packed, err := reset.Message().MarshalPacked()
	reset.Release()

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: greaterThan reset frame marshal failed",
			err,
		))

		return
	}

	if _, err = greaterThan.Right.Write(packed); err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: greaterThan reset stage failed",
			err,
		))
	}
}

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
		rightScalars = []float64{constant.value}
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

/*
Equal compares input samples against a wired operand stage.
*/
type Equal struct {
	Right io.ReadWriteCloser
}

func (equal Equal) Match(artifact *datura.Artifact) bool {
	rootKey := datura.Peek[string](artifact, "root")
	inputKeys := datura.Peek[[]string](artifact, "inputs")

	if rootKey == "" || len(inputKeys) == 0 {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"logic: equal wire required",
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
				"logic: equal input sample is non-finite",
				nil,
			))

			return false
		}

		samples[index] = sample
	}

	var rightScalars []float64

	if constant, isConstant := equal.Right.(*Constant); isConstant {
		rightScalars = []float64{constant.value}
	} else {
		rightScratch := datura.Acquire("logic-operand-right", datura.APPJSON)

		packed, err := artifact.Message().MarshalPacked()

		if err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: equal operand marshal failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		if _, err = rightScratch.Write(packed); err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: equal operand write failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		if err = transport.NewFlipFlop(rightScratch, equal.Right); err != nil {
			errnie.Error(errnie.Err(
				errnie.IO,
				"logic: equal operand flipFlop failed",
				err,
			))
			rightScratch.Release()

			return false
		}

		rightKeys := datura.Peek[[]string](rightScratch, "inputs")

		if len(rightKeys) == 0 {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"logic: equal operand wire required",
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
			if sample != right {
				return false
			}
		}
	}

	return true
}

func (equal Equal) ResetOperands() {
	if equal.Right == nil {
		return
	}

	reset := datura.Acquire("logic-reset", datura.APPJSON)
	reset.Poke(1, "reset")

	packed, err := reset.Message().MarshalPacked()
	reset.Release()

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: equal reset frame marshal failed",
			err,
		))

		return
	}

	if _, err = equal.Right.Write(packed); err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: equal reset stage failed",
			err,
		))
	}
}
