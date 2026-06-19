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
	artifact    *datura.Artifact
	rules       Rules
	output      float64
	inboundWire []byte
}

/*
NewCircuit returns a branching stage for the given ordered rules.
*/
func NewCircuit(rules Rules) *Circuit {
	return &Circuit{
		artifact: datura.Acquire("circuit", datura.APPJSON).RetainStageAttributes(),
		rules:    rules,
	}
}

func (circuit *Circuit) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](circuit.artifact, "output") == nil

	circuit.artifact.Clear("sample")

	n, err := circuit.artifact.Write(p)

	if bootstrap {
		circuit.artifact.Clear("output")
	}

	circuit.inboundWire = append(circuit.inboundWire[:0], p...)

	return n, err
}

func (circuit *Circuit) Read(p []byte) (int, error) {
	inbound := circuit.inboundWire
	inboundOK := len(inbound) > 0

	if !inboundOK {
		inbound, inboundOK = artifactBytes(circuit.artifact)
	}

	for _, rule := range circuit.rules {
		if !rule.Condition.Match(circuit.artifact) {
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

		circuit.artifact.Poke(datura.Map[float64]{"value": circuit.output}, "output")

		return circuit.artifact.Read(p)
	}

	circuit.artifact.Poke(datura.Map[float64]{"value": circuit.output}, "output")

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
	circuit.inboundWire = circuit.inboundWire[:0]

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

func isConstantStage(stage io.ReadWriter) bool {
	_, isConstant := stage.(*Constant)

	return isConstant
}
