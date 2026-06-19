package causal

import (
	"github.com/theapemachine/datura"
)

/*
Ladder evaluates Judea Pearl's ladder of causation over tabular rows on the artifact.
*/
type Ladder struct {
	artifact *datura.Artifact
}

/*
NewLadder returns a ladder stage.
*/
func NewLadder() *Ladder {
	return &Ladder{
		artifact: datura.Acquire("ladder", datura.APPJSON),
	}
}

func (ladder *Ladder) Write(p []byte) (int, error) {
	return ladder.artifact.Write(p)
}

func (ladder *Ladder) Read(p []byte) (int, error) {
	rows, ok := tableRows(ladder.artifact)

	if !ok {
		return ladder.artifact.Read(p)
	}

	target := int(datura.Peek[float64](ladder.artifact, "config", "target"))
	minHistory := int(datura.Peek[float64](ladder.artifact, "config", "minHistory"))

	if minHistory <= 0 {
		minHistory = 12
	}

	if len(rows) < minHistory {
		return ladder.artifact.Read(p)
	}

	table, err := newNodeTable(rows, target, minHistory)

	if err != nil {
		return ladder.artifact.Read(p)
	}

	inverted := datura.Peek[float64](ladder.artifact, "output", "value") != 0
	contagion := datura.Peek[float64](ladder.artifact, "paired")

	if contagion == 0 {
		contagion = datura.Peek[float64](ladder.artifact, "output", "contagion")
	}

	condition := datura.Peek[float64](ladder.artifact, "output", "condition")
	treatment := int(datura.Peek[float64](ladder.artifact, "config", "treatmentNormal"))
	controls := intSlice(datura.Peek[[]float64](ladder.artifact, "config", "controlsNormal"))

	if inverted {
		treatment = int(datura.Peek[float64](ladder.artifact, "config", "treatmentInverted"))
		controls = intSlice(datura.Peek[[]float64](ladder.artifact, "config", "controlsInverted"))
	}

	bandwidth := datura.Peek[float64](ladder.artifact, "config", "kernelBandwidth")

	if bandwidth <= 0 {
		bandwidth = deriveBandwidth(rows, int(datura.Peek[float64](ladder.artifact, "config", "treatmentNormal")))
	}

	if bandwidth <= 0 {
		return ladder.artifact.Read(p)
	}

	association, assocErr := table.association(treatment)

	if assocErr != nil {
		association = 0
	}

	intervention, intervErr := table.kernelBackdoorEffect(treatment, bandwidth, controls...)

	if intervErr != nil {
		intervention = 0
	}

	raw := 0.0
	uplift := 0.0

	if intervention > 0 {
		predictors := append(append([]int(nil), controls...), treatment)
		nonLinear, fitOK := fitNonLinearTable(table, predictors)
		raw = intervention
		currentRow := rows[len(rows)-1]
		interventionLevel := datura.Peek[float64](ladder.artifact, "config", "interventionLevel")

		if interventionLevel <= 0 {
			level, levelErr := table.percentile(treatment, 0.75)

			if levelErr == nil {
				interventionLevel = level
			}
		}

		if fitOK {
			upliftValue, upliftErr := nonLinear.counterfactualUplift(currentRow, treatment, interventionLevel)

			if upliftErr == nil {
				uplift = upliftValue
			}
		}

		if !fitOK {
			linear, linearErr := table.fitLinearModel(predictors...)

			if linearErr == nil {
				upliftValue, upliftErr := linear.counterfactualUplift(currentRow, treatment, interventionLevel)

				if upliftErr == nil {
					uplift = upliftValue
				}
			}
		}
	}

	invertedValue := 0.0

	if inverted {
		invertedValue = 1
	}

	ladder.artifact.Poke(datura.Map[float64]{
		"value":        raw,
		"association":  association,
		"intervention": intervention,
		"uplift":       uplift,
		"contagion":    contagion,
		"condition":    condition,
		"inverted":     invertedValue,
	}, "output")

	return ladder.artifact.Read(p)
}

func (ladder *Ladder) Close() error {
	return nil
}
