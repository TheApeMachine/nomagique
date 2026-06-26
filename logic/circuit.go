package logic

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique"
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
}

/*
NewCircuit returns a branching stage for the given ordered rules.
*/
func NewCircuit(artifact *datura.Artifact, rules Rules) *Circuit {
	return &Circuit{
		artifact: artifact,
		rules:    rules,
	}
}

func (circuit *Circuit) Read(payload []byte) (int, error) {
	state := datura.Acquire("circuit-state", datura.APPJSON)

	if _, err := state.Unpack(circuit.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logic: state write failed",
			err,
		))
	}

	defer state.Release()

	rootKey := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logic: circuit root required",
			nil,
		))
	}

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logic: circuit inputs required",
			nil,
		))
	}

	matched := false

	for _, rule := range circuit.rules {
		if !rule.Condition.Match(state) {
			continue
		}

		scratch := datura.Acquire("circuit-operand", datura.APPJSON)

		packed, err := state.Message().MarshalPacked()

		if err != nil {
			scratch.Release()

			return 0, errnie.Error(errnie.Err(
				errnie.IO,
				"logic: circuit scratch marshal failed",
				err,
			))
		}

		if _, err = scratch.Unpack(packed); err != nil {
			scratch.Release()

			return 0, errnie.Error(errnie.Err(
				errnie.IO,
				"logic: circuit scratch write failed",
				err,
			))
		}

		if datura.Peek[string](scratch, "root") == "output" {
			if rootKey == "" || len(inputs) == 0 {
				scratch.Release()

				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"logic: circuit output stamp wire required",
					nil,
				))
			}

			scratch.Poke(rootKey, "root")
			scratch.Poke(inputs, "inputs")
		}

		if err = nomagique.RoundTripArtifact(scratch, rule.Then); err != nil {
			scratch.Release()

			return 0, errnie.Error(errnie.Err(
				errnie.IO,
				"logic: circuit flipFlop through then stage failed",
				err,
			))
		}

		packed, err = scratch.Message().MarshalPacked()
		scratch.Release()

		if err != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.IO,
				"logic: circuit scratch marshal failed",
				err,
			))
		}

		if _, err = state.Unpack(packed); err != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.IO,
				"logic: circuit state write failed",
				err,
			))
		}

		matched = true
		break
	}

	if !matched {
		state.MergeOutput("value", 0)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
	}

	return state.PackInto(payload)
}

func (circuit *Circuit) Write(payload []byte) (int, error) {
	inbound := datura.Acquire("logic-inbound", datura.APPJSON)

	if _, err := inbound.Unpack(payload); err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"logic: circuit inbound write failed",
			err,
		))
	}

	if datura.Peek[float64](inbound, "reset") != 0 {
		for _, rule := range circuit.rules {
			rule.Condition.ResetOperands()

			if rule.Then == nil {
				continue
			}

			reset := datura.Acquire("logic-reset", datura.APPJSON)
			reset.Poke(1, "reset")

			packed, err := reset.Message().MarshalPacked()
			reset.Release()

			if err != nil {
				errnie.Error(errnie.Err(
					errnie.IO,
					"logic: reset frame marshal failed",
					err,
				))

				continue
			}

			if _, err = rule.Then.Write(packed); err != nil {
				errnie.Error(errnie.Err(
					errnie.IO,
					"logic: circuit reset stage failed",
					err,
				))
			}
		}
	}

	inbound.Release()

	circuit.artifact.WithPayload(payload)
	return len(payload), nil
}

func (circuit *Circuit) Close() error {
	return nil
}
