package causal

import (
	"math"
	"sort"

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

func (regime *Regime) Read(p []byte) (int, error) {
	state := datura.Acquire("regime-state", datura.APPJSON)

	if _, err := state.Unpack(regime.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: state write failed",
			err,
		))
	}

	rows, err := tableRows(state)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal regime: missing table rows",
			err,
		))
	}

	target := int(datura.Peek[float64](regime.artifact, "target"))
	minHistory := int(datura.Peek[float64](regime.artifact, "minHistory"))

	if minHistory <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal regime: minHistory required",
			nil,
		))
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
	contagionBreakPoint := datura.Peek[float64](regime.artifact, "contagionBreak")

	if contagionBreakPoint <= 0 {
		contagionBreakPoint = dynamicContagionBreak(
			rows,
			target,
			intSlice(datura.Peek[[]float64](regime.artifact, "contagionSkip")),
		)
	}

	contagionBreak := contagionBreakPoint > 0 && contagion > contagionBreakPoint

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
	return state.PackInto(p)
}

func (regime *Regime) Write(p []byte) (int, error) {
	regime.artifact.WithPayload(p)
	return len(p), nil
}

func (regime *Regime) Close() error {
	return nil
}

func dynamicContagionBreak(rows [][]float64, target int, skip []int) float64 {
	samples := make([]float64, 0, len(rows))

	for _, row := range rows {
		for nodeIndex, sample := range row {
			if nodeIndex == target || containsIndex(skip, nodeIndex) {
				continue
			}

			samples = append(samples, math.Abs(sample))
		}
	}

	if len(samples) == 0 {
		return 0
	}

	center := median(samples)
	deviations := make([]float64, 0, len(samples))

	for _, sample := range samples {
		deviations = append(deviations, math.Abs(sample-center))
	}

	return center + median(deviations)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2

	if len(sorted)%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}

	return sorted[middle]
}
