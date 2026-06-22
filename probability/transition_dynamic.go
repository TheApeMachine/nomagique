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

func (transition *Transition) Write(payload []byte) (int, error) {
	transition.artifact.WithPayload(payload)
	return len(payload), nil
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

	if _, err := state.Write(wire); err != nil {
		state.Release()

		return 0, err
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
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
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

	matrix := transitionMatrixFromPayload(transition.artifact, numStates, alpha)
	observed := matrix.PadObserved(probabilities, 0)
	surprise, surpriseErr := matrix.Surprise(observed)

	if surpriseErr != nil {
		return 0, surpriseErr
	}

	categoryIndex := transitionCategory(state)

	if categoryIndex >= 1 && categoryIndex <= numStates {
		matrix.Update(categoryIndex - 1)
	}

	pokeTransitionMatrix(transition.artifact, matrix)
	state.MergeOutput("value", surprise)
	state.MergeOutput("category", categoryIndex)
	state.MergeOutput("probabilities", probabilities)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (transition *Transition) Close() error {
	return nil
}

func transitionMatrixFromPayload(
	artifact *datura.Artifact,
	numStates int,
	alpha float64,
) *TransitionMatrix {
	matrix := NewTransitionMatrix(numStates, alpha)
	rawAttributes, err := artifact.Attributes()

	if err != nil || len(rawAttributes) == 0 {
		return matrix
	}

	countsNode, err := sonic.Get(rawAttributes, "transition", "counts")

	if err != nil || !countsNode.Exists() {
		return matrix
	}

	rawCounts, err := countsNode.ArrayUseNode()

	if err != nil || len(rawCounts) != numStates*numStates {
		return matrix
	}

	for row := range numStates {
		for column := range numStates {
			index := row*numStates + column
			sample, sampleErr := rawCounts[index].Float64()

			if sampleErr != nil {
				return matrix
			}

			matrix.counts[row][column] = sample
		}
	}

	lastCategoryNode, err := sonic.Get(rawAttributes, "transition", "lastCategory")

	if err != nil || !lastCategoryNode.Exists() {
		return matrix
	}

	lastCategory, err := lastCategoryNode.Float64()

	if err != nil {
		return matrix
	}

	matrix.lastCategory = int(lastCategory)

	return matrix
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
	body := datura.As[datura.Map[any]](state)

	if body == nil {
		return nil
	}

	output, ok := body["output"].(map[string]any)

	if !ok {
		return nil
	}

	raw, ok := output["probabilities"].([]any)

	if !ok {
		return nil
	}

	probabilities := make([]float64, len(raw))

	for index, sample := range raw {
		numeric, numericOK := sample.(float64)

		if !numericOK {
			return nil
		}

		probabilities[index] = numeric
	}

	return probabilities
}

func transitionCategory(state *datura.Artifact) int {
	body := datura.As[datura.Map[any]](state)

	if body == nil {
		return 0
	}

	output, ok := body["output"].(map[string]any)

	if !ok {
		return 0
	}

	numeric, ok := output["category"].(float64)

	if !ok {
		return 0
	}

	return int(numeric)
}
