package logic

import "github.com/theapemachine/errnie"

/*
Rule binds one condition to the stage that runs when it matches.
*/
type Rule struct {
	Condition Condition
	Then      Stage
}

/*
Rules is an ordered circuit program. The first matching rule wins.
*/
type Rules []Rule

/*
Stage transforms a typed observation.
*/
type Stage interface {
	Measure(observation Observation) (Observation, error)
}

/*
Circuit walks rules and routes observations through the first matching stage.
*/
type Circuit struct {
	rules Rules
}

/*
NewCircuit returns a typed branching circuit for ordered rules.
*/
func NewCircuit(rules Rules) *Circuit {
	return &Circuit{
		rules: rules,
	}
}

/*
Measure returns the first matching rule output, or zero when no rule matches.
*/
func (circuit *Circuit) Measure(observation Observation) (Observation, error) {
	if len(observation.Values) == 0 {
		return Observation{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"logic: observation values required",
			nil,
		))
	}

	for _, rule := range circuit.rules {
		if rule.Condition == nil || !rule.Condition.Match(observation) {
			continue
		}

		if rule.Then == nil {
			return Observation{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"logic: matching rule stage required",
				nil,
			))
		}

		return rule.Then.Measure(observation)
	}

	return NewObservation(0), nil
}
