package logic

import "github.com/theapemachine/datura"

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
