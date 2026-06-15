package algorithm

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/causal"
)

/*
Backdoor estimates a linear backdoor-adjusted treatment effect from aligned node streams.
*/
type Backdoor struct {
	artifact    *datura.Artifact
	target      int
	treatment   int
	controls    []int
	streams     [][]float64
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
	streams [][]float64,
	minRows int,
) *Backdoor {
	if minRows <= 0 {
		minRows = 12
	}

	return &Backdoor{
		artifact:  datura.Acquire("backdoor", datura.Artifact_Type_json),
		target:    target,
		treatment: treatment,
		controls:  append([]int(nil), controls...),
		streams:   streams,
		minRows:   minRows,
	}
}

func (backdoor *Backdoor) Write(p []byte) (int, error) {
	return backdoor.artifact.Write(p)
}

func (backdoor *Backdoor) Read(p []byte) (int, error) {
	rehydrateArtifact(&backdoor.artifact, "backdoor", datura.Artifact_Type_json)

	table, ok := backdoor.nodeTable()

	if !ok {
		backdoor.clearReadings()

		return backdoor.artifact.Read(p)
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
	out := encodePayload(effect)
	_ = backdoor.artifact.SetPayload(out)

	return backdoor.artifact.Read(p)
}

func (backdoor *Backdoor) Close() error {
	return nil
}

/*
Association returns the raw Pearson association from the last Read call.
*/
func (backdoor *Backdoor) Association() float64 {
	return backdoor.association
}

/*
Effect returns the backdoor-adjusted effect from the last Read call.
*/
func (backdoor *Backdoor) Effect() float64 {
	return backdoor.effect
}

/*
ConditionNumber returns the treatment-target pair condition estimate.
*/
func (backdoor *Backdoor) ConditionNumber() float64 {
	return backdoor.condition
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
