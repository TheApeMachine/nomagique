package probability

import (
	"github.com/theapemachine/datura"
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
	artifact.Inspect("probability", "transition", "NewTransitionSurprise()")

	return &Transition{artifact: artifact}
}

func (transition *Transition) Write(payload []byte) (int, error) {
	transition.artifact.WithPayload(payload)
	return len(payload), nil
}

func (transition *Transition) Read(payload []byte) (int, error) {
	state := datura.Acquire("transition-state", datura.APPJSON)
	state.Inspect("probability", "transition", "Read()", "p")

	if _, err := state.Write(transition.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

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
		return state.Read(payload)
	}

	probabilities := datura.Peek[[]float64](state, "output", "probabilities")

	if len(probabilities) == 0 {
		return state.Read(payload)
	}

	matrix := transitionMatrixFromPayload(transition.artifact, numStates, alpha)
	observed := matrix.PadObserved(probabilities, 0)
	surprise, surpriseErr := matrix.Surprise(observed)

	if surpriseErr != nil {
		return state.Read(payload)
	}

	categoryIndex := int(datura.Peek[float64](state, "output", "category"))

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
	counts := datura.Peek[[]float64](artifact, "transition", "counts")

	if len(counts) == numStates*numStates {
		for row := range numStates {
			for column := range numStates {
				matrix.counts[row][column] = counts[row*numStates+column]
			}
		}
	}

	matrix.lastCategory = int(datura.Peek[float64](artifact, "transition", "lastCategory"))

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
