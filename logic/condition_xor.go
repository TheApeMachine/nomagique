package logic

/*
Xor matches when exactly one nested condition matches.
*/
type Xor []Condition

func (xorCondition Xor) Match(observation Observation) bool {
	matches := 0

	for _, operand := range xorCondition {
		if operand != nil && operand.Match(observation) {
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
