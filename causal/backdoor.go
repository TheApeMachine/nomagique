package causal

import (
	"errors"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal backdoor: missing table rows",
			errors.New("causal: table rows missing"),
		))
	}

	target := int(datura.Peek[float64](backdoor.artifact, "target"))
	treatment := int(datura.Peek[float64](backdoor.artifact, "treatment"))
	controls := intSlice(datura.Peek[[]float64](backdoor.artifact, "controls"))
	minRows := int(datura.Peek[float64](backdoor.artifact, "minHistory"))

	if minRows <= 0 {
		minRows = len(rows)
	}

	table, err := newNodeTable(rows, target, minRows)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal backdoor: table construction failed",
			err,
		))
	}

	association, err := table.association(treatment)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal backdoor: association failed",
			err,
		))
	}

	effect, err := table.backdoorEffect(treatment, controls...)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal backdoor: effect estimation failed",
			err,
		))
	}

	condition, err := table.pairConditionNumber(treatment, target)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal backdoor: pair condition failed",
			err,
		))
	}

	state.MergeOutput("value", effect)
	state.MergeOutput("association", association)
	state.MergeOutput("effect", effect)
	state.MergeOutput("condition", condition)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value", "association", "effect", "condition"})
	return state.Read(p)
}

func (backdoor *Backdoor) Close() error {
	return nil
}
