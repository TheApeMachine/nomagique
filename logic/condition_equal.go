package logic

/*
Equal compares every observation value against every right operand.
*/
type Equal struct {
	Right ValueSource
}

func (equal Equal) Match(observation Observation) bool {
	right := rightValues(equal.Right, observation)

	if len(observation.Values) == 0 || len(right) == 0 {
		return false
	}

	for _, sample := range observation.Values {
		for _, operand := range right {
			if sample != operand {
				return false
			}
		}
	}

	return true
}

func (equal Equal) ResetOperands() {}

func rightValues(source ValueSource, observation Observation) []float64 {
	if source == nil {
		return nil
	}

	return source.Values(observation)
}
