package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

/*
NewPearl returns the Judea Pearl ladder-of-causation pipeline.
*/
func NewPearl() io.ReadWriteCloser {
	return nomagique.Number(
		statistic.NewPanel(),
		statistic.NewMedian(),
		causal.NewContagion(),
		equation.NewRegimeLadder(),
	)
}
