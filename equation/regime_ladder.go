package equation

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/causal"
)

/*
NewRegimeLadder composes regime selection, hysteresis smoothing, and ladder evaluation.
*/
func NewRegimeLadder(config *datura.Artifact) io.ReadWriteCloser {
	return nomagique.Number(
		causal.NewRegime(config),
		adaptive.NewHysteresis(config),
		causal.NewLadder(config),
	)
}
