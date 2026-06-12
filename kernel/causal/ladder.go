package causal

import "math"

/*
Outcome is the Pearl-ladder read plus margins that separate categories.
*/
type Outcome struct {
	Raw          float64
	Reason       string
	Intervention float64
	Association  float64
	Uplift       float64
	Inverted     bool
	Contagion    float64
	Condition    float64
}

/*
Evaluate runs the Pearl ladder on a node table and current observation row.
*/
func Evaluate(
	nodeTable NodeTable,
	currentRow []float64,
	contagion float64,
	config LadderConfig,
	tracker *RegimeTracker,
) Outcome {
	if len(nodeTable.rows) < config.MinHistory {
		return Outcome{}
	}

	roles, inverted, condition := SelectRolesWithTracker(
		nodeTable, contagion, config, tracker, len(nodeTable.rows),
	)
	suffix := ""

	if inverted {
		suffix = "_regime_inversion"
	}

	bandwidth := config.KernelBandwidth

	if bandwidth <= 0 {
		bandwidth = 0.35
	}

	association, assocErr := nodeTable.Association(roles.Treatment)

	if assocErr != nil {
		association = 0
	}

	intervention, intervErr := nodeTable.KernelBackdoorEffect(
		roles.Treatment, bandwidth, roles.Controls...,
	)

	if intervErr != nil {
		intervention = 0
	}

	outcome := Outcome{
		Intervention: intervention,
		Association:  association,
		Inverted:     inverted,
		Contagion:    contagion,
		Condition:    condition,
	}

	if intervention <= 0 {
		return outcome
	}

	model, fitOK := FitNonLinearTable(nodeTable, roles.Predictors())

	if !fitOK {
		outcome.Raw = intervention
		outcome.Reason = "intervention" + suffix

		return outcome
	}

	interventionLevel := config.InterventionLevel

	if interventionLevel <= 0 {
		level, levelErr := nodeTable.Percentile(roles.Treatment, 0.75)

		if levelErr != nil {
			level = 0
		}

		interventionLevel = level
	}

	uplift, upliftErr := model.CounterfactualUplift(
		currentRow, roles.Treatment, interventionLevel,
	)

	if upliftErr != nil {
		uplift = 0
	}

	outcome.Uplift = uplift

	if uplift <= 0 {
		outcome.Raw = intervention
		outcome.Reason = "intervention" + suffix

		return outcome
	}

	confoundFraction := config.ConfoundFraction

	if confoundFraction <= 0 {
		confoundFraction = 0.25
	}

	confounded := math.Abs(intervention-association) > math.Abs(association)*confoundFraction
	outcome.Reason = "intervention" + suffix

	if confounded {
		outcome.Reason = "counterfactual_like" + suffix
	}

	outcome.Raw = intervention

	return outcome
}
