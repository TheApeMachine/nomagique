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
		artifact: datura.Acquire("circuit", datura.APPJSON),
		rules:    rules,
	}
}

func (circuit *Circuit) Write(p []byte) (int, error) {
	if inboundReset(p) {
		circuit.output = 0
		circuit.inboundWire = circuit.inboundWire[:0]

		for _, rule := range circuit.rules {
			resetCondition(rule.Condition)
			writeResetToStage(rule.Then)
		}
	}

	n, err := circuit.artifact.Write(p)

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

func isConstantStage(stage io.ReadWriteCloser) bool {
	_, isConstant := stage.(*Constant)

	return isConstant
}
