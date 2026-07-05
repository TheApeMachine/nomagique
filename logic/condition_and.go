package logic

/*
And matches when every nested condition matches.
*/
type And []Condition

func (andCondition And) Match(observation Observation) bool {
	if len(andCondition) == 0 {
		return false
	}

	for _, operand := range andCondition {
		if operand == nil || !operand.Match(observation) {
			return false
		}
	}

	return true
}

func (andCondition And) ResetOperands() {
	for _, operand := range andCondition {
		operand.ResetOperands()
	}
}
