package logic

import (
	"io"

	"github.com/theapemachine/datura"
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

	inbound, inboundOK := artifactBytes(artifact)

	if !inboundOK {
		return false
	}

	value, valueOK := readOperand(trueCondition.Stage, inbound)

	return valueOK && truthy(value)
}

func (trueCondition True) ResetOperands() {
	writeResetToStage(trueCondition.Stage)
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
GreaterThan compares a boundary sample against a wired operand stage.
*/
type GreaterThan struct {
	Right io.ReadWriteCloser
}

func (greaterThan GreaterThan) Match(artifact *datura.Artifact) bool {
	sample, sampleOK := boundarySample(artifact)

	if !sampleOK {
		return false
	}

	inbound, inboundOK := artifactBytes(artifact)

	if !inboundOK {
		return false
	}

	right, rightOK := readOperand(greaterThan.Right, inbound)

	return rightOK && sample > right
}

func (greaterThan GreaterThan) ResetOperands() {
	writeResetToStage(greaterThan.Right)
}

/*
LessThan compares a boundary sample against a wired operand stage.
*/
type LessThan struct {
	Right io.ReadWriteCloser
}

func (lessThan LessThan) Match(artifact *datura.Artifact) bool {
	sample, sampleOK := boundarySample(artifact)

	if !sampleOK {
		return false
	}

	inbound, inboundOK := artifactBytes(artifact)

	if !inboundOK {
		return false
	}

	right, rightOK := readOperand(lessThan.Right, inbound)

	return rightOK && sample < right
}

func (lessThan LessThan) ResetOperands() {
	writeResetToStage(lessThan.Right)
}

/*
Equal compares a boundary sample against a wired operand stage.
*/
type Equal struct {
	Right io.ReadWriteCloser
}

func (equal Equal) Match(artifact *datura.Artifact) bool {
	sample, sampleOK := boundarySample(artifact)

	if !sampleOK {
		return false
	}

	inbound, inboundOK := artifactBytes(artifact)

	if !inboundOK {
		return false
	}

	right, rightOK := readOperand(equal.Right, inbound)

	return rightOK && sample == right
}

func (equal Equal) ResetOperands() {
	writeResetToStage(equal.Right)
}

func truthy(value float64) bool {
	return value != 0
}
