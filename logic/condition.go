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
	Reset() error
}

/*
True always matches when Operand is true, otherwise checks a wired stage.
*/
type True struct {
	Operand bool
	Stage   io.ReadWriter
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

func (trueCondition True) Reset() error {
	return resetStage(trueCondition.Stage)
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

func (andCondition And) Reset() error {
	for _, operand := range andCondition {
		if resetErr := operand.Reset(); resetErr != nil {
			return resetErr
		}
	}

	return nil
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

func (orCondition Or) Reset() error {
	for _, operand := range orCondition {
		if resetErr := operand.Reset(); resetErr != nil {
			return resetErr
		}
	}

	return nil
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

func (notCondition Not) Reset() error {
	return notCondition.Operand.Reset()
}

/*
Xor matches when an odd number of nested conditions match.
*/
type Xor []Condition

func (xorCondition Xor) Match(artifact *datura.Artifact) bool {
	matchCount := 0

	for _, operand := range xorCondition {
		if operand.Match(artifact) {
			matchCount++
		}
	}

	return matchCount%2 == 1
}

func (xorCondition Xor) Reset() error {
	for _, operand := range xorCondition {
		if resetErr := operand.Reset(); resetErr != nil {
			return resetErr
		}
	}

	return nil
}

/*
GreaterThan matches when the carried signal exceeds Right.
*/
type GreaterThan struct {
	Right io.ReadWriter
}

func (greaterThan GreaterThan) Match(artifact *datura.Artifact) bool {
	left, leftOK := boundarySample(artifact)

	if !leftOK {
		return false
	}

	inbound, inboundOK := artifactBytes(artifact)

	if !inboundOK {
		return false
	}

	right, rightOK := readOperand(greaterThan.Right, inbound)

	if !rightOK {
		return false
	}

	return left > right
}

func (greaterThan GreaterThan) Reset() error {
	return resetStage(greaterThan.Right)
}

/*
LessThan matches when the carried signal is below Right.
*/
type LessThan struct {
	Right io.ReadWriter
}

func (lessThan LessThan) Match(artifact *datura.Artifact) bool {
	left, leftOK := boundarySample(artifact)

	if !leftOK {
		return false
	}

	inbound, inboundOK := artifactBytes(artifact)

	if !inboundOK {
		return false
	}

	right, rightOK := readOperand(lessThan.Right, inbound)

	if !rightOK {
		return false
	}

	return left < right
}

func (lessThan LessThan) Reset() error {
	return resetStage(lessThan.Right)
}

/*
Equal matches when the carried signal equals Right.
*/
type Equal struct {
	Right io.ReadWriter
}

func (equal Equal) Match(artifact *datura.Artifact) bool {
	left, leftOK := boundarySample(artifact)

	if !leftOK {
		return false
	}

	inbound, inboundOK := artifactBytes(artifact)

	if !inboundOK {
		return false
	}

	right, rightOK := readOperand(equal.Right, inbound)

	if !rightOK {
		return false
	}

	return left == right
}

func (equal Equal) Reset() error {
	return resetStage(equal.Right)
}

func truthy(value float64) bool {
	return value != 0
}
