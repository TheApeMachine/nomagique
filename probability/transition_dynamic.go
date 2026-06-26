package probability

import (
	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Transition scores transition surprisal from classifier probabilities and records
the selected category into retained transition state.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Transition struct {
	artifact *datura.Artifact
}

/*
NewTransitionSurprise returns a transition surprisal stage wired from schema
attributes numStates and alpha.
*/
func NewTransitionSurprise(artifact *datura.Artifact) *Transition {
	return &Transition{artifact: artifact}
}

func (transition *Transition) Read(payload []byte) (int, error) {
	state := datura.Acquire("transition-state", datura.APPJSON)
	wire := transition.artifact.DecryptPayload()

	if len(wire) == 0 {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition stage missing inbound wire",
			nil,
		))
	}

	if _, err := state.Unpack(wire); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: state write failed",
			err,
		))
	}

	defer state.Release()

	numStates := int(datura.Peek[float64](transition.artifact, "numStates"))
	alpha := datura.Peek[float64](transition.artifact, "alpha")
	inboundAlpha := datura.Peek[float64](state, "alpha")

	if inboundAlpha > 0 {
		alpha = inboundAlpha
		transition.artifact.Poke(alpha, "alpha")
	}

	if datura.Peek[float64](state, "reset") != 0 {
		transition.artifact.WithAttributes(datura.Map[any]{
			"numStates": float64(numStates),
			"alpha":     alpha,
		})
		state.MergeOutput("value", 0)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
		return state.PackInto(payload)
	}

	if numStates <= 0 || alpha <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: numStates and alpha must be positive",
			nil,
		))
	}

	probabilities := transitionProbabilities(state)

	if len(probabilities) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: probabilities required",
			nil,
		))
	}

	matrix, err := transitionMatrixFromPayload(transition.artifact, numStates, alpha)

	if err != nil {
		return 0, err
	}

	observed, err := matrix.PadObserved(probabilities, alpha)

	if err != nil {
		return 0, err
	}
	surprise, err := matrix.Surprise(observed)

	if err != nil {
		return 0, err
	}

	categoryIndex := transitionCategory(state)

	if categoryIndex >= 1 && categoryIndex <= numStates {
		matrix.Update(categoryIndex - 1)
	}

	pokeTransitionMatrix(transition.artifact, matrix)
	state.MergeOutput("value", surprise)
	state.MergeOutput("category", categoryIndex)
	state.MergeOutput("probabilities", probabilities)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	return state.PackInto(payload)
}

func (transition *Transition) Write(payload []byte) (int, error) {
	transition.artifact.WithPayload(payload)
	return len(payload), nil
}

func (transition *Transition) Close() error {
	return nil
}

func transitionMatrixFromPayload(
	artifact *datura.Artifact,
	numStates int,
	alpha float64,
) (*TransitionMatrix, error) {
	matrix := NewTransitionMatrix(numStates, alpha)
	rawAttributes, err := artifact.Attributes()

	if err != nil || len(rawAttributes) == 0 {
		return matrix, nil
	}

	countsNode, err := sonic.Get(rawAttributes, "transition", "counts")

	if err != nil || !countsNode.Exists() {
		return matrix, nil
	}

	rawCounts, err := countsNode.ArrayUseNode()

	if err != nil {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: unable to read counts",
			err,
		))
	}

	if len(rawCounts) != numStates*numStates {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: counts length mismatch",
			nil,
		))
	}

	for row := range numStates {
		for column := range numStates {
			index := row*numStates + column
			sample, err := rawCounts[index].Float64()

			if err != nil {
				return nil, errnie.Error(errnie.Err(
					errnie.Validation,
					"transition: counts entry is non-numeric",
					err,
				))
			}

			matrix.counts[row][column] = sample
		}
	}

	lastCategoryNode, err := sonic.Get(rawAttributes, "transition", "lastCategory")

	if err != nil || !lastCategoryNode.Exists() {
		return matrix, nil
	}

	lastCategory, err := lastCategoryNode.Float64()

	if err != nil {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: lastCategory is non-numeric",
			err,
		))
	}

	matrix.lastCategory = int(lastCategory)

	return matrix, nil
}

func pokeTransitionMatrix(artifact *datura.Artifact, matrix *TransitionMatrix) {
	if matrix == nil {
		return
	}

	flat := make([]float64, 0, matrix.numStates*matrix.numStates)

	for row := range matrix.counts {
		flat = append(flat, matrix.counts[row]...)
	}

	artifact.Poke(flat, "transition", "counts")
	artifact.Poke(float64(matrix.lastCategory), "transition", "lastCategory")
}

func transitionProbabilities(state *datura.Artifact) []float64 {
	raw, err := sonic.Get(state.DecryptPayload(), "output", "probabilities")

	if err != nil || !raw.Exists() {
		return nil
	}

	values, err := raw.ArrayUseNode()

	if err != nil {
		return nil
	}

	probabilities := make([]float64, len(values))

	for index, sample := range values {
		numeric, err := sample.Float64()

		if err != nil {
			return nil
		}

		probabilities[index] = numeric
	}

	return probabilities
}

func transitionCategory(state *datura.Artifact) int {
	category, err := sonic.Get(state.DecryptPayload(), "output", "category")

	if err != nil || !category.Exists() {
		return 0
	}

	value, err := category.Float64()

	if err != nil {
		return 0
	}

	return int(value)
}
