package causal

import (
	"errors"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Regime selects normal or inverted DAG roles from contagion and pair condition.
The constructor artifact holds config; Write buffers inbound table wire on its payload.
*/
type Regime struct {
	artifact *datura.Artifact
}

/*
NewRegime returns a regime stage wired from config attributes on the artifact.
*/
func NewRegime(artifact *datura.Artifact) *Regime {
	return &Regime{
		artifact: artifact,
	}
}

func (regime *Regime) Write(p []byte) (int, error) {
	regime.artifact.WithPayload(p)
	return len(p), nil
}

func (regime *Regime) Read(p []byte) (int, error) {
	state := datura.Acquire("regime-state", datura.APPJSON)

	if _, err := state.Write(regime.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("causal", "regime", "Read()", "p")

	rows, ok := tableRows(state)

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal regime: missing table rows",
			errors.New("causal: table rows missing"),
		))
	}

	target := int(datura.Peek[float64](regime.artifact, "target"))
	minHistory := int(datura.Peek[float64](regime.artifact, "minHistory"))

	if minHistory <= 0 {
		minHistory = len(rows)
	}

	table, err := newNodeTable(rows, target, minHistory)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal regime: table construction failed",
			err,
		))
	}

	contagion := datura.Peek[float64](state, "paired")
	contagionBreak := datura.Peek[float64](regime.artifact, "contagionBreak") > 0 &&
		contagion >= datura.Peek[float64](regime.artifact, "contagionBreak")

	condition := 0.0
	conditionBreak := false
	conditionSwitch := datura.Peek[float64](regime.artifact, "conditionSwitch")

	if conditionSwitch > 0 {
		pairCondition, err := table.pairConditionNumber(
			int(datura.Peek[float64](regime.artifact, "conditionLeft")),
			int(datura.Peek[float64](regime.artifact, "conditionRight")),
		)

		if err != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal regime: pair condition failed",
				err,
			))
		}

		condition = pairCondition
		conditionBreak = math.IsInf(pairCondition, 1) || pairCondition >= conditionSwitch
	}

	rawInverted := 0.0

	if conditionBreak || contagionBreak {
		rawInverted = 1
	}

	state.MergeOutput("value", condition)
	state.MergeOutput("condition", condition)
	state.MergeOutput("rawInverted", rawInverted)
	state.Poke("output", "root")
	state.Poke([]string{"value", "condition", "rawInverted"}, "inputs")
	return state.Read(p)
}

func (regime *Regime) Close() error {
	return nil
}
