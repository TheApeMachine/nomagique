package causal

import (
	"github.com/theapemachine/datura"
)

/*
Backdoor estimates a linear backdoor-adjusted treatment effect from table.* on the artifact.
The constructor artifact holds config; Write buffers inbound table wire on its payload.
*/
type Backdoor struct {
	artifact *datura.Artifact
}

/*
NewBackdoor returns a backdoor stage wired from config attributes on the artifact.
*/
func NewBackdoor(artifact *datura.Artifact) *Backdoor {
	artifact.Inspect("causal", "backdoor", "NewBackdoor()")

	return &Backdoor{
		artifact: artifact,
	}
}

func (backdoor *Backdoor) Write(p []byte) (int, error) {
	backdoor.artifact.WithPayload(p)
	return len(p), nil
}

func (backdoor *Backdoor) Read(p []byte) (int, error) {
	state := datura.Acquire("backdoor-state", datura.APPJSON)
	state.Inspect("causal", "backdoor", "Read()", "p")

	if _, err := state.Write(backdoor.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	rows, ok := tableRows(state)

	if !ok {
		state.MergeOutput("value", 0.0)
		return state.Read(p)
	}

	target := int(datura.Peek[float64](backdoor.artifact, "config", "target"))
	treatment := int(datura.Peek[float64](backdoor.artifact, "config", "treatment"))
	controls := intSlice(datura.Peek[[]float64](backdoor.artifact, "config", "controls"))
	minRows := int(datura.Peek[float64](backdoor.artifact, "config", "minHistory"))

	if minRows <= 0 {
		minRows = len(rows)
	}

	table, err := newNodeTable(rows, target, minRows)

	if err != nil {
		state.MergeOutput("value", 0.0)
		return state.Read(p)
	}

	association, err := table.association(treatment)

	if err != nil {
		association = 0
	}

	effect, err := table.backdoorEffect(treatment, controls...)

	if err != nil {
		effect = 0
	}

	condition, err := table.pairConditionNumber(treatment, target)

	if err != nil {
		condition = 0
	}

	state.MergeOutput("value", effect)
	state.MergeOutput("association", association)
	state.MergeOutput("effect", effect)
	state.MergeOutput("condition", condition)
	return state.Read(p)
}

func (backdoor *Backdoor) Close() error {
	return nil
}
