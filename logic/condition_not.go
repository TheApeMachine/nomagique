package logic

/*
Not inverts a nested condition.
*/
type Not struct {
	Operand Condition
}

func (notCondition Not) Match(observation Observation) bool {
	if notCondition.Operand == nil {
		return true
	}

	return !notCondition.Operand.Match(observation)
}

func (notCondition Not) ResetOperands() {
	if notCondition.Operand != nil {
		notCondition.Operand.ResetOperands()
	}
}
