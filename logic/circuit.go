package logic

import (
	"io"

	"github.com/theapemachine/datura"
)

/*
Rule binds one condition to the stage that runs when it matches.
*/
type Rule struct {
	Condition Condition
	Then      io.ReadWriteCloser
}

/*
Rules is an ordered circuit program. The first matching rule wins.
*/
type Rules []Rule

/*
Circuit walks rules and routes observations through the first matching Then stage.
*/
type Circuit struct {
	artifact *datura.Artifact
	rules    Rules
	output   float64
}

/*
NewCircuit returns a branching stage for the given ordered rules.
*/
func NewCircuit(rules Rules) *Circuit {
	return &Circuit{
		artifact: datura.Acquire("circuit", datura.APPJSON),
		rules:    rules,
	}
}

func (circuit *Circuit) Write(p []byte) (int, error) {
	if inboundReset(p) {
		circuit.output = 0

		for _, rule := range circuit.rules {
			resetCondition(rule.Condition)
			writeResetToStage(rule.Then)
		}
	}

	circuit.artifact.WithPayload(p)
	return len(p), nil
}

func (circuit *Circuit) Read(p []byte) (int, error) {
	state := datura.Acquire("circuit-state", datura.APPJSON)

	if _, err := state.Write(circuit.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	inbound, inboundOK := artifactBytes(state)

	for _, rule := range circuit.rules {
		if !rule.Condition.Match(state) {
			continue
		}

		if constant, isConstant := rule.Then.(*Constant); isConstant {
			circuit.output = constant.value
		}

		if !isConstantStage(rule.Then) && inboundOK {
			value, valueOK := readOperand(rule.Then, inbound)

			if valueOK {
				circuit.output = value
			}
		}

		state.MergeOutput("value", circuit.output)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(p)
	}

	state.MergeOutput("value", circuit.output)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(p)
}

func (circuit *Circuit) Close() error {
	return nil
}

func isConstantStage(stage io.ReadWriteCloser) bool {
	_, isConstant := stage.(*Constant)

	return isConstant
}
