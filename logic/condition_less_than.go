package logic

/*
LessThan compares every observation value against every right operand.
*/
type LessThan struct {
	Right ValueSource
}

func (lessThan LessThan) Match(observation Observation) bool {
	right := rightValues(lessThan.Right, observation)

	if len(observation.Values) == 0 || len(right) == 0 {
		return false
	}

	for _, sample := range observation.Values {
		for _, operand := range right {
			if sample >= operand {
				return false
			}
		}
	}

	return true
}

func (lessThan LessThan) ResetOperands() {}
