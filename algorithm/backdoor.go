package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/causal"
)

/*
NewBackdoor returns a backdoor estimator over tabular rows on the artifact.
*/
func NewBackdoor(artifact *datura.Artifact) io.ReadWriteCloser {
	return nomagique.Number(causal.NewBackdoor(artifact))
}
