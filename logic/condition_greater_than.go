package logic

/*
GreaterThan compares every observation value against every right operand.
*/
type GreaterThan struct {
	Right ValueSource
}

func (greaterThan GreaterThan) Match(observation Observation) bool {
	right := rightValues(greaterThan.Right, observation)

	if len(observation.Values) == 0 || len(right) == 0 {
		return false
	}

	for _, sample := range observation.Values {
		for _, operand := range right {
			if sample <= operand {
				return false
			}
		}
	}

	return true
}

func (greaterThan GreaterThan) ResetOperands() {}
