package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

/*
NewPearl returns the Judea Pearl ladder-of-causation pipeline.
*/
func NewPearl(config *datura.Artifact) io.ReadWriteCloser {
	return nomagique.Number(
		statistic.NewPanel(),
		statistic.NewMedian(),
		causal.NewContagion(config),
		equation.NewRegimeLadder(config),
	)
}
