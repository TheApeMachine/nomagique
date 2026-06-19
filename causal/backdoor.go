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
		artifact: datura.Acquire("backdoor", datura.APPJSON),
	}
}

func (backdoor *Backdoor) Write(p []byte) (int, error) {
	return backdoor.artifact.Write(p)
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

	table, err := newNodeTable(rows, target, minRows)

	if err != nil {
		backdoor.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return backdoor.artifact.Read(p)
	}

	association, assocErr := table.association(treatment)

	if assocErr != nil {
		association = 0
	}

	effect, effectErr := table.backdoorEffect(treatment, controls...)

	if effectErr != nil {
		effect = 0
	}

	condition, conditionErr := table.pairConditionNumber(treatment, target)

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
