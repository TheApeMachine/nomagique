package probability

import (
	"github.com/theapemachine/datura"
)

/*
Transition scores transition surprisal from classifier probabilities and optionally
records the next category index.
*/
type Transition struct {
	artifact *datura.Artifact
	matrix   *TransitionMatrix
}

/*
NewTransitionSurprise returns a transition surprisal stage for io.ReadWriter pipelines.
*/
func NewTransitionSurprise(numStates int, alpha float64) *Transition {
	return &Transition{
		artifact: datura.Acquire("transition", datura.Artifact_Type_json),
		matrix:   NewTransitionMatrix(numStates, alpha),
	}
}

func (transition *Transition) Write(p []byte) (int, error) {
	return transition.artifact.Write(p)
}

func (transition *Transition) Read(p []byte) (int, error) {
	rehydrateArtifact(&transition.artifact, "transition", datura.Artifact_Type_json)

	if transition == nil || transition.matrix == nil {
		return transition.artifact.Read(p)
	}

	probabilities := datura.Peek[[]float64](transition.artifact, "classifier.probabilities")

	if len(probabilities) == 0 || len(probabilities) < transition.matrix.numStates {
		return transition.artifact.Read(p)
	}

	observed := probabilities[:transition.matrix.numStates]
	surprise, surpriseErr := transition.matrix.Surprise(observed)

	if surpriseErr == nil {
		out := encodePayload(surprise)
		_ = transition.artifact.SetPayload(out)
		pokeFloat(transition.artifact, "transition.surprise", surprise)

		categoryIndex, categoryOK := datura.PeekOK[int](transition.artifact, "classifier.category")

		if categoryOK && categoryIndex >= 1 && categoryIndex <= transition.matrix.numStates {
			transition.matrix.Update(categoryIndex - 1)
		}
	}

	return transition.artifact.Read(p)
}

func (transition *Transition) Close() error {
	return nil
}

/*
SetSmoothingAlpha replaces the Dirichlet smoothing prior used by PadObserved.
*/
func (transition *Transition) SetSmoothingAlpha(alpha float64) {
	if transition == nil || transition.matrix == nil || alpha <= 0 {
		return
	}

	transition.matrix.smoothingAlpha = alpha
}

/*
Observe scores surprisal from classifier probabilities and records categoryIndex.
*/
func (transition *Transition) Observe(
	probabilities []float64,
	categoryIndex int,
) (float64, error) {
	if transition == nil || transition.matrix == nil || len(probabilities) == 0 {
		return 0, nil
	}

	observed := transition.matrix.PadObserved(probabilities, 0)
	surprise, surpriseErr := transition.matrix.Surprise(observed)

	if surpriseErr != nil {
		return 0, surpriseErr
	}

	if categoryIndex >= 1 && categoryIndex <= transition.matrix.numStates {
		transition.matrix.Update(categoryIndex - 1)
	}

	return surprise, nil
}

/*
Reset clears transition counts back to the smoothing prior.
*/
func (transition *Transition) Reset() error {
	if transition == nil || transition.matrix == nil {
		return nil
	}

	transition.matrix.Reset()

	return nil
}
