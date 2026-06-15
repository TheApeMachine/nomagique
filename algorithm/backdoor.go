package algorithm

import (
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/core"
)

/*
Backdoor estimates a linear backdoor-adjusted treatment effect from aligned node streams.
*/
type Backdoor[T ~float64] struct {
	target      int
	treatment   int
	controls    []int
	streams     [][]float64
	minRows     int
	association float64
	effect      float64
	condition   float64
	output      core.Scalar[T]
}

/*
NewBackdoor creates a backdoor estimator over per-node streams.
*/
func NewBackdoor[T ~float64](
	target, treatment int,
	controls []int,
	streams [][]float64,
	minRows int,
) *Backdoor[T] {
	if minRows <= 0 {
		minRows = 12
	}

	return &Backdoor[T]{
		target:    target,
		treatment: treatment,
		controls:  append([]int(nil), controls...),
		streams:   streams,
		minRows:   minRows,
	}
}

/*
Observe rebuilds the node table and returns the backdoor effect magnitude.
*/
func (backdoor *Backdoor[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	table, ok := backdoor.nodeTable()

	if !ok {
		backdoor.clearReadings()

		return backdoor.output
	}

	association, assocErr := table.Association(backdoor.treatment)

	if assocErr != nil {
		association = 0
	}

	effect, effectErr := table.BackdoorEffect(backdoor.treatment, backdoor.controls...)

	if effectErr != nil {
		effect = 0
	}

	condition, conditionErr := table.PairConditionNumber(backdoor.treatment, backdoor.target)

	if conditionErr != nil {
		condition = 0
	}

	backdoor.association = association
	backdoor.effect = effect
	backdoor.condition = condition
	backdoor.output = core.Scalar[T](T(effect))

	return backdoor.output
}

/*
Association returns the raw Pearson association from the last Observe call.
*/
func (backdoor *Backdoor[T]) Association() core.Scalar[T] {
	return core.Scalar[T](T(backdoor.association))
}

/*
Effect returns the backdoor-adjusted effect from the last Observe call.
*/
func (backdoor *Backdoor[T]) Effect() core.Scalar[T] {
	return core.Scalar[T](T(backdoor.effect))
}

/*
ConditionNumber returns the treatment-target pair condition estimate.
*/
func (backdoor *Backdoor[T]) ConditionNumber() core.Scalar[T] {
	return core.Scalar[T](T(backdoor.condition))
}

/*
Reset clears derived state.
*/
func (backdoor *Backdoor[T]) Reset() error {
	backdoor.clearReadings()
	backdoor.output = core.Scalar[T](0)

	return nil
}

func (backdoor *Backdoor[T]) nodeTable() (causal.NodeTable, bool) {
	rows, ok := zipNodeRows(backdoor.streams)

	if !ok {
		return causal.NodeTable{}, false
	}

	table, err := causal.NewNodeTable(rows, backdoor.target, backdoor.minRows)

	if err != nil {
		return causal.NodeTable{}, false
	}

	return table, true
}

func (backdoor *Backdoor[T]) clearReadings() {
	backdoor.association = 0
	backdoor.effect = 0
	backdoor.condition = 0
}
