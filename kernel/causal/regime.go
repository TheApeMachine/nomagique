package causal

import "math"

/*
Roles assigns DAG node roles for one structural regime.
*/
type Roles struct {
	Treatment int
	Controls  []int
	Label     string
}

/*
LadderConfig holds thresholds and node indices for Pearl ladder role selection.
*/
type LadderConfig struct {
	TreatmentNormal   int
	ControlsNormal    []int
	TreatmentInverted int
	ControlsInverted  []int
	ConditionLeft     int
	ConditionRight    int
	ContagionBreak    float64
	ConditionSwitch   float64
	KernelBandwidth   float64
	ConfoundFraction  float64
	InterventionLevel float64
	MinHistory        int
}

/*
Predictors returns treatment plus controls for model fitting.
*/
func (roles Roles) Predictors() []int {
	return append(append([]int(nil), roles.Controls...), roles.Treatment)
}

/*
SelectRoles chooses normal or inverted roles from contagion and pair condition.
*/
func SelectRoles(
	nodeTable NodeTable,
	contagion float64,
	config LadderConfig,
) (roles Roles, inverted bool, condition float64) {
	normal := Roles{
		Treatment: config.TreatmentNormal,
		Controls:  append([]int(nil), config.ControlsNormal...),
		Label:     "normal",
	}

	contagionBreak := config.ContagionBreak > 0 && contagion >= config.ContagionBreak
	conditionBreak := false

	if config.ConditionSwitch > 0 {
		pairCondition, condErr := nodeTable.PairConditionNumber(
			config.ConditionLeft, config.ConditionRight,
		)

		if condErr == nil {
			condition = pairCondition
			conditionBreak = math.IsInf(pairCondition, 1) ||
				pairCondition >= config.ConditionSwitch
		}
	}

	if conditionBreak || contagionBreak {
		return Roles{
			Treatment: config.TreatmentInverted,
			Controls:  append([]int(nil), config.ControlsInverted...),
			Label:     "inverted",
		}, true, condition
	}

	return normal, false, condition
}

/*
SelectRolesWithTracker applies hysteresis before returning roles.
*/
func SelectRolesWithTracker(
	nodeTable NodeTable,
	contagion float64,
	config LadderConfig,
	tracker *RegimeTracker,
	historyLen int,
) (roles Roles, inverted bool, condition float64) {
	_, rawInverted, condition := SelectRoles(nodeTable, contagion, config)

	if tracker == nil {
		if rawInverted {
			return Roles{
				Treatment: config.TreatmentInverted,
				Controls:  append([]int(nil), config.ControlsInverted...),
				Label:     "inverted",
			}, true, condition
		}

		return Roles{
			Treatment: config.TreatmentNormal,
			Controls:  append([]int(nil), config.ControlsNormal...),
			Label:     "normal",
		}, false, condition
	}

	inverted = tracker.Apply(rawInverted, DeriveRegimeHysteresisSamples(historyLen))

	if inverted {
		return Roles{
			Treatment: config.TreatmentInverted,
			Controls:  append([]int(nil), config.ControlsInverted...),
			Label:     "inverted",
		}, true, condition
	}

	return Roles{
		Treatment: config.TreatmentNormal,
		Controls:  append([]int(nil), config.ControlsNormal...),
		Label:     "normal",
	}, false, condition
}
