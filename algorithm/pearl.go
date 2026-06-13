package algorithm

import (
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/core"
)

/*
Pearl implements Judea Pearl's ladder of causation over a tabular structural model.
*/
type Pearl struct {
	target    int
	config    causal.LadderConfig
	streams   []core.Numbers
	current   core.Numbers
	contagion core.Number
	tracker   *causal.RegimeTracker
	weights   core.Numbers
	outcome   causal.Outcome
}

/*
NewPearl creates a Pearl dynamic over aligned per-node streams and a contagion scalar.
*/
func NewPearl(
	target int,
	config causal.LadderConfig,
	streams []core.Numbers,
	contagion core.Number,
	weights core.Numbers,
) *Pearl {
	if config.MinHistory <= 0 {
		config.MinHistory = 12
	}

	config = applyDerivedLadderConfig(config, streams)

	return &Pearl{
		target:    target,
		config:    config,
		streams:   streams,
		contagion: contagion,
		tracker:   causal.NewRegimeTracker(),
		weights:   weights,
	}
}

/*
Observe evaluates the ladder and returns the intervention effect magnitude.
When inputs are provided they form the current observation row.
*/
func (pearl *Pearl) Observe(inputs ...core.Number) core.Float64 {
	if len(inputs) > 0 {
		pearl.current = core.Numbers(inputs)
	}

	table, currentRow, ok := pearl.tableFromStreams()

	if !ok {
		return 0
	}

	contagion := float64(pearl.contagion.Observe())
	pearl.outcome = causal.Evaluate(
		table, currentRow, contagion, pearl.config, pearl.tracker,
	)

	return core.Float64(pearl.outcome.Raw)
}

/*
Association returns the Pearson association between treatment and target.
*/
func (pearl *Pearl) Association() core.Float64 {
	return core.Float64(pearl.outcome.Association)
}

/*
Intervention returns the kernel backdoor intervention estimate.
*/
func (pearl *Pearl) Intervention() core.Float64 {
	return core.Float64(pearl.outcome.Intervention)
}

/*
Uplift returns the nonlinear counterfactual uplift when available.
*/
func (pearl *Pearl) Uplift() core.Float64 {
	return core.Float64(pearl.outcome.Uplift)
}

/*
RegimeInverted reports whether the inverted role set is active.
*/
func (pearl *Pearl) RegimeInverted() bool {
	return pearl.outcome.Inverted
}

/*
Outcome returns the full ladder decomposition from the last Observe call.
*/
func (pearl *Pearl) Outcome() causal.Outcome {
	return pearl.outcome
}

/*
Reset clears derived state.
*/
func (pearl *Pearl) Reset() error {
	pearl.weights = nil
	pearl.outcome = causal.Outcome{}
	pearl.tracker = causal.NewRegimeTracker()

	return nil
}

func (pearl *Pearl) tableFromStreams() (causal.NodeTable, []float64, bool) {
	rows, ok := zipNodeRows(pearl.streams)

	if !ok {
		return causal.NodeTable{}, nil, false
	}

	currentRow := pearl.currentRow(rows)

	table, err := causal.NewNodeTable(rows, pearl.target, pearl.config.MinHistory)

	if err != nil {
		return causal.NodeTable{}, nil, false
	}

	return table, currentRow, true
}

func (pearl *Pearl) currentRow(rows [][]float64) []float64 {
	if pearl.current != nil {
		current := nomagique.Samples(pearl.current)

		if len(current) == len(rows[0]) {
			return current
		}
	}

	if len(rows) == 0 {
		return nil
	}

	return rows[len(rows)-1]
}

func zipNodeRows(streams []core.Numbers) ([][]float64, bool) {
	if len(streams) == 0 {
		return nil, false
	}

	first := nomagique.Samples(streams[0])
	rowCount := len(first)

	if rowCount == 0 {
		return nil, false
	}

	rows := make([][]float64, rowCount)

	for rowIndex := range rows {
		rows[rowIndex] = make([]float64, len(streams))

		for nodeIndex, stream := range streams {
			samples := nomagique.Samples(stream)

			if len(samples) != rowCount {
				return nil, false
			}

			rows[rowIndex][nodeIndex] = samples[rowIndex]
		}
	}

	return rows, true
}
