package causal

import (
	"github.com/theapemachine/datura"
)

/*
Backdoor estimates a linear backdoor-adjusted treatment effect from table.* on the artifact.
*/
type Backdoor struct {
	artifact *datura.Artifact
}

/*
NewBackdoor returns a backdoor stage reading config and table rows from the artifact.
*/
func NewBackdoor() *Backdoor {
	return &Backdoor{
		artifact: datura.Acquire("backdoor", datura.APPJSON).RetainStageAttributes(),
	}
}

func (backdoor *Backdoor) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](backdoor.artifact, "output") == nil

	backdoor.artifact.Clear("sample")
	backdoor.artifact.Clear("batch")

	n, err := backdoor.artifact.Write(p)

	if bootstrap {
		backdoor.artifact.Clear("output")
	}

	return n, err
}

func (backdoor *Backdoor) Read(p []byte) (int, error) {
	rows, ok := tableRows(backdoor.artifact)

	if !ok {
		backdoor.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return backdoor.artifact.Read(p)
	}

	target := int(datura.Peek[float64](backdoor.artifact, "config", "target"))
	treatment := int(datura.Peek[float64](backdoor.artifact, "config", "treatment"))
	controls := intSlice(datura.Peek[[]float64](backdoor.artifact, "config", "controls"))
	minRows := int(datura.Peek[float64](backdoor.artifact, "config", "minHistory"))

	if minRows <= 0 {
		minRows = 12
	}

	table, err := NewNodeTable(rows, target, minRows)

	if err != nil {
		backdoor.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return backdoor.artifact.Read(p)
	}

	association, assocErr := table.Association(treatment)

	if assocErr != nil {
		association = 0
	}

	effect, effectErr := table.BackdoorEffect(treatment, controls...)

	if effectErr != nil {
		effect = 0
	}

	condition, conditionErr := table.PairConditionNumber(treatment, target)

	if conditionErr != nil {
		condition = 0
	}

	backdoor.artifact.Poke(datura.Map[float64]{
		"value":       effect,
		"association": association,
		"effect":      effect,
		"condition":   condition,
	}, "output")

	return backdoor.artifact.Read(p)
}

func (backdoor *Backdoor) Close() error {
	return nil
}

/*
WithConfig stores target, treatment, controls, and minHistory on the stage artifact.
*/
func (backdoor *Backdoor) WithConfig(
	target, treatment int,
	controls []int,
	minHistory int,
) *Backdoor {
	if backdoor == nil {
		return nil
	}

	controlValues := make([]float64, len(controls))

	for index, control := range controls {
		controlValues[index] = float64(control)
	}

	backdoor.artifact.
		Poke(float64(target), "config", "target").
		Poke(float64(treatment), "config", "treatment").
		Poke(controlValues, "config", "controls").
		Poke(float64(minHistory), "config", "minHistory")

	return backdoor
}
