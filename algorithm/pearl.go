package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

type (
	// NodeRing is a causal history primitive re-exported for preset wiring.
	NodeRing = causal.NodeRing
)

// NewNodeRing returns a bounded multi-node history accumulator.
var NewNodeRing = causal.NewNodeRing

/*
Pearl is the Judea Pearl ladder-of-causation algorithm as one io.ReadWriter pipeline.
*/
type Pearl struct {
	pipeline io.ReadWriter
	nodeRing *causal.NodeRing
	ladder   *causal.Ladder
	panel    *statistic.Panel
}

/*
NewPearl returns the Pearl preset: node history, cross-section panel, contagion,
regime path, ladder rungs, and category classification.
*/
func NewPearl() *Pearl {
	pearl := &Pearl{
		panel:  statistic.NewPanel(),
		ladder: causal.NewLadder(),
	}

	pearl.rebuild()

	return pearl
}

func (pearl *Pearl) Write(p []byte) (int, error) {
	return pearl.pipeline.Write(p)
}

func (pearl *Pearl) Read(p []byte) (int, error) {
	return pearl.pipeline.Read(p)
}

func (pearl *Pearl) Close() error {
	return nil
}

/*
SetNodes binds the live node ring prepended to the pipeline.
*/
func (pearl *Pearl) SetNodes(nodeRing *NodeRing) {
	pearl.nodeRing = nodeRing
	pearl.rebuild()
}

func (pearl *Pearl) rebuild() {
	stages := []io.ReadWriter{
		pearl.panel,
		statistic.NewMedian(),
		causal.NewContagion(),
		equation.NewRegimeLadder(pearl.ladder),
	}

	if pearl.nodeRing != nil {
		stages = append([]io.ReadWriter{pearl.nodeRing}, stages...)
	}

	pearl.pipeline = nomagique.Number(stages...)
}
