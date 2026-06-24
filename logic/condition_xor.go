package logic

import "github.com/theapemachine/datura"

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
