package logic

/*
Or matches when any nested condition matches.
*/
type Or []Condition

func (orCondition Or) Match(observation Observation) bool {
	for _, operand := range orCondition {
		if operand != nil && operand.Match(observation) {
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
