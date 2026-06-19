package probability

import (
	"github.com/theapemachine/datura"
)

/*
Transition scores transition surprisal from classifier probabilities and records
the selected category into retained transition state.
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

func (transition *Transition) Write(p []byte) (int, error) {
	inbound := datura.Acquire("transition-inbound", datura.APPJSON)
	_, _ = inbound.Write(p)

	probabilities := datura.Peek[[]float64](inbound, "output", "probabilities")
	category := datura.Peek[float64](inbound, "output", "category")
	reset := attributeKeyPresent(inbound, "reset")
	inboundAlpha := datura.Peek[float64](inbound, "alpha")

	transitionCounts := datura.Peek[[]float64](transition.artifact, "transition", "counts")
	lastCategory := datura.Peek[float64](transition.artifact, "transition", "lastCategory")

	n, err := transition.artifact.Write(p)

	if err != nil {
		return n, err
	}

	if inboundAlpha > 0 {
		transition.artifact.Poke(inboundAlpha, "alpha")
	}

	if reset {
		return n, nil
	}

	if len(probabilities) > 0 {
		transition.artifact.Poke(probabilities, "output", "probabilities")
	}

	if category > 0 {
		transition.artifact.Poke(category, "output", "category")
	}

	if transitionCounts != nil {
		transition.artifact.Poke(transitionCounts, "transition", "counts")
	}

	if lastCategory > 0 || transitionCounts != nil {
		transition.artifact.Poke(lastCategory, "transition", "lastCategory")
	}

	return n, nil
}

func (transition *Transition) Read(p []byte) (int, error) {
	numStates := int(datura.Peek[float64](transition.artifact, "numStates"))
	alpha := datura.Peek[float64](transition.artifact, "alpha")

	if numStates <= 0 || alpha <= 0 {
		return transition.artifact.Read(p)
	}

	matrix := transitionMatrixFromPayload(transition.artifact, numStates, alpha)
	probabilities := datura.Peek[[]float64](transition.artifact, "output", "probabilities")

	if len(probabilities) == 0 {
		return transition.artifact.Read(p)
	}

	observed := matrix.PadObserved(probabilities, 0)
	surprise, surpriseErr := matrix.Surprise(observed)

	if surpriseErr != nil {
		return transition.artifact.Read(p)
	}

	transition.artifact.Poke(surprise, "output", "value")

	categoryIndex := int(datura.Peek[float64](transition.artifact, "output", "category"))

	if categoryIndex >= 1 && categoryIndex <= numStates {
		matrix.Update(categoryIndex - 1)
	}

	pokeTransitionMatrix(transition.artifact, matrix)

	return transition.artifact.Read(p)
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
