package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/causal"
)

/*
NewBackdoor returns a backdoor estimator over tabular rows on the artifact.
*/
func NewBackdoor(target, treatment int, controls []int, minRows int) io.ReadWriter {
	stage := causal.NewBackdoor().WithConfig(target, treatment, controls, minRows)

	return nomagique.Number(stage)
}
