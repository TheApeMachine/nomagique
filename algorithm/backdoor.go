package algorithm

import (
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/core"
)

/*
Backdoor estimates a linear backdoor-adjusted treatment effect from aligned node streams.
*/
type Backdoor struct {
	target      int
	treatment   int
	controls    []int
	streams     []core.Numbers
	minRows     int
	association float64
	effect      float64
	condition   float64
}

/*
NewBackdoor creates a backdoor estimator over per-node streams.
*/
func NewBackdoor(
	target, treatment int,
	controls []int,
	streams []core.Numbers,
	minRows int,
) *Backdoor {
	if minRows <= 0 {
		minRows = 12
	}

	return &Backdoor{
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
func (backdoor *Backdoor) Observe(_ ...core.Number) core.Float64 {
	table, ok := backdoor.nodeTable()

	if !ok {
		backdoor.clearReadings()

		return 0
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

	return core.Float64(effect)
}

/*
Association returns the raw Pearson association from the last Observe call.
*/
func (backdoor *Backdoor) Association() core.Float64 {
	return core.Float64(backdoor.association)
}

/*
Effect returns the backdoor-adjusted effect from the last Observe call.
*/
func (backdoor *Backdoor) Effect() core.Float64 {
	return core.Float64(backdoor.effect)
}

/*
ConditionNumber returns the treatment-target pair condition estimate.
*/
func (backdoor *Backdoor) ConditionNumber() core.Float64 {
	return core.Float64(backdoor.condition)
}

/*
Reset clears derived state.
*/
func (backdoor *Backdoor) Reset() error {
	backdoor.clearReadings()

	return nil
}

func (backdoor *Backdoor) nodeTable() (causal.NodeTable, bool) {
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

func (backdoor *Backdoor) clearReadings() {
	backdoor.association = 0
	backdoor.effect = 0
	backdoor.condition = 0
}
