package logic

/*
True matches non-zero observations or a non-zero stage output.
*/
type True struct {
	Stage Stage
}

func (trueCondition True) Match(observation Observation) bool {
	if trueCondition.Stage != nil {
		output, err := trueCondition.Stage.Measure(observation)

		if err != nil {
			return false
		}

		observation = output
	}

	for _, value := range observation.Values {
		if value != 0 {
			return true
		}
	}

	return false
}

func (trueCondition True) ResetOperands() {}
