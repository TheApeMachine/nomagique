package logic

import "github.com/theapemachine/datura"

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
