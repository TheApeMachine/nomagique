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
	Then      io.ReadWriter
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
		artifact: datura.Acquire("circuit", datura.Artifact_Type_json),
		rules:    rules,
	}
}

func (circuit *Circuit) Write(p []byte) (int, error) {
	return circuit.artifact.Write(p)
}

func (circuit *Circuit) Read(p []byte) (int, error) {
	inbound, inboundOK := artifactBytes(circuit.artifact)

	for _, rule := range circuit.rules {
		if !rule.Condition.Match(circuit.artifact) {
			continue
		}

		if constant, isConstant := rule.Then.(*Constant); isConstant {
			circuit.output = constant.value

			putFloat64Payload(&circuit.artifact, "circuit", circuit.output)

			return circuit.artifact.Read(p)
		}

		if inboundOK {
			value, valueOK := readOperand(rule.Then, inbound)

			if valueOK {
				circuit.output = value
			}
		}

		putFloat64Payload(&circuit.artifact, "circuit", circuit.output)

		return circuit.artifact.Read(p)
	}

	putFloat64Payload(&circuit.artifact, "circuit", circuit.output)

	return circuit.artifact.Read(p)
}

func (circuit *Circuit) Close() error {
	return nil
}

/*
Reset clears derived state on every rule.
*/
func (circuit *Circuit) Reset() error {
	circuit.output = 0

	for _, rule := range circuit.rules {
		if resetErr := rule.Condition.Reset(); resetErr != nil {
			return resetErr
		}

		if resetErr := resetStage(rule.Then); resetErr != nil {
			return resetErr
		}
	}

	return nil
}
