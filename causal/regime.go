package causal

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Regime selects normal or inverted DAG roles from contagion and pair condition.
*/
type Regime struct {
	artifact *datura.Artifact
}

/*
NewRegime returns a regime stage.
*/
func NewRegime() *Regime {
	return &Regime{
		artifact: datura.Acquire("regime", datura.APPJSON),
	}
}

func (regime *Regime) Write(p []byte) (int, error) {
	return regime.artifact.Write(p)
}

func (regime *Regime) Read(p []byte) (int, error) {
	rows, ok := tableRows(regime.artifact)

	if !ok {
		regime.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return regime.artifact.Read(p)
	}

	target := int(datura.Peek[float64](regime.artifact, "config", "target"))
	minHistory := int(datura.Peek[float64](regime.artifact, "config", "minHistory"))

	if minHistory <= 0 {
		minHistory = 12
	}

	table, err := newNodeTable(rows, target, minHistory)

	if err != nil {
		regime.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return regime.artifact.Read(p)
	}

	contagion := datura.Peek[float64](regime.artifact, "paired")

	if contagion == 0 {
		contagion = datura.Peek[float64](regime.artifact, "output", "value")
	}

	contagionBreak := datura.Peek[float64](regime.artifact, "config", "contagionBreak") > 0 &&
		contagion >= datura.Peek[float64](regime.artifact, "config", "contagionBreak")

	condition := 0.0
	conditionBreak := false
	conditionSwitch := datura.Peek[float64](regime.artifact, "config", "conditionSwitch")

	if conditionSwitch > 0 {
		pairCondition, condErr := table.pairConditionNumber(
			int(datura.Peek[float64](regime.artifact, "config", "conditionLeft")),
			int(datura.Peek[float64](regime.artifact, "config", "conditionRight")),
		)

		if condErr == nil {
			condition = pairCondition
			conditionBreak = math.IsInf(pairCondition, 1) || pairCondition >= conditionSwitch
		}
	}

	rawInverted := 0.0

	if conditionBreak || contagionBreak {
		rawInverted = 1
	}

	regime.artifact.Poke(rawInverted, "sample")
	regime.artifact.Poke(datura.Map[float64]{
		"value":       condition,
		"condition":   condition,
		"rawInverted": rawInverted,
	}, "output")

	return regime.artifact.Read(p)
}

func (regime *Regime) Close() error {
	return nil
}
