package algorithm

import (
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/core"
)

/*
Pearl implements Judea Pearl's ladder of causation over a tabular structural model.
*/
type Pearl[T ~float64] struct {
	target    int
	config    causal.LadderConfig
	streams   [][]float64
	current   []float64
	contagion core.Number[T]
	tracker   *causal.RegimeTracker
	weights   []float64
	outcome   causal.Outcome
	output    core.Scalar[T]
}

/*
NewPearl creates a Pearl dynamic over aligned per-node streams and a contagion scalar.
*/
func NewPearl[T ~float64](
	target int,
	config causal.LadderConfig,
	streams [][]float64,
	contagion core.Number[T],
	weights []float64,
) *Pearl[T] {
	if config.MinHistory <= 0 {
		config.MinHistory = 12
	}

	config = applyDerivedLadderConfig(config, streams)

	return &Pearl[T]{
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
func (pearl *Pearl[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) > 0 {
		pearl.current = sampleBatch[T](inputs...)
	}

	table, currentRow, ok := pearl.tableFromStreams()

	if !ok {
		pearl.output = core.Scalar[T](0)

		return pearl.output
	}

	contagion := float64(pearl.contagion.Observe())
	pearl.outcome = causal.Evaluate(
		table, currentRow, contagion, pearl.config, pearl.tracker,
	)

	pearl.output = core.Scalar[T](T(pearl.outcome.Raw))

	return pearl.output
}

/*
Association returns the Pearson association between treatment and target.
*/
func (pearl *Pearl[T]) Association() core.Scalar[T] {
	return core.Scalar[T](T(pearl.outcome.Association))
}

/*
Intervention returns the kernel backdoor intervention estimate.
*/
func (pearl *Pearl[T]) Intervention() core.Scalar[T] {
	return core.Scalar[T](T(pearl.outcome.Intervention))
}

/*
Uplift returns the nonlinear counterfactual uplift when available.
*/
func (pearl *Pearl[T]) Uplift() core.Scalar[T] {
	return core.Scalar[T](T(pearl.outcome.Uplift))
}

/*
RegimeInverted reports whether the inverted role set is active.
*/
func (pearl *Pearl[T]) RegimeInverted() bool {
	return pearl.outcome.Inverted
}

/*
Outcome returns the full ladder decomposition from the last Observe call.
*/
func (pearl *Pearl[T]) Outcome() causal.Outcome {
	return pearl.outcome
}

/*
Reset clears derived state.
*/
func (pearl *Pearl[T]) Reset() error {
	pearl.weights = nil
	pearl.current = nil
	pearl.outcome = causal.Outcome{}
	pearl.output = core.Scalar[T](0)
	pearl.tracker = causal.NewRegimeTracker()

	return nil
}

func (pearl *Pearl[T]) tableFromStreams() (causal.NodeTable, []float64, bool) {
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

func (pearl *Pearl[T]) currentRow(rows [][]float64) []float64 {
	if len(pearl.current) == len(rows[0]) {
		return pearl.current
	}

	if len(rows) == 0 {
		return nil
	}

	return rows[len(rows)-1]
}
