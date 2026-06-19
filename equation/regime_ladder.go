package equation

import (
	"io"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/causal"
)

/*
NewRegimeLadder composes regime selection, hysteresis smoothing, and ladder evaluation.
*/
func NewRegimeLadder(ladderStage *causal.Ladder) io.ReadWriter {
	return nomagique.Number(
		causal.NewRegime(),
		adaptive.NewHysteresis(),
		ladderStage,
	)
}
