package logic

import "github.com/theapemachine/datura"

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
