package causal

import (
	"github.com/theapemachine/datura"
)

/*
Ladder evaluates Judea Pearl's ladder of causation over tabular rows on the artifact.
The constructor artifact holds config; Write buffers inbound table wire on its payload.
*/
type Ladder struct {
	artifact *datura.Artifact
}

/*
NewLadder returns a ladder stage wired from config attributes on the artifact.
*/
func NewLadder(artifact *datura.Artifact) *Ladder {
	artifact.Inspect("causal", "ladder", "NewLadder()")

	return &Ladder{
		artifact: artifact,
	}
}

func (ladder *Ladder) Write(p []byte) (int, error) {
	ladder.artifact.WithPayload(p)
	return len(p), nil
}

func (ladder *Ladder) Read(p []byte) (int, error) {
	state := datura.Acquire("ladder-state", datura.APPJSON)
	state.Inspect("causal", "ladder", "Read()", "p")

	if _, err := state.Write(ladder.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	rows, ok := tableRows(state)

	if !ok {
		return state.Read(p)
	}

	target := int(datura.Peek[float64](ladder.artifact, "config", "target"))
	minHistory := int(datura.Peek[float64](ladder.artifact, "config", "minHistory"))

	if minHistory <= 0 {
		minHistory = len(rows)
	}

	if len(rows) < minHistory {
		return state.Read(p)
	}

	table, err := newNodeTable(rows, target, minHistory)

	if err != nil {
		return state.Read(p)
	}

	inverted := datura.Peek[float64](state, "output", "value") != 0
	contagion := datura.Peek[float64](state, "paired")

	if contagion == 0 {
		contagion = datura.Peek[float64](state, "output", "contagion")
	}

	condition := datura.Peek[float64](state, "output", "condition")
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
		return state.Read(p)
	}

	association, err := table.association(treatment)

	if err != nil {
		association = 0
	}

	intervention, err := table.kernelBackdoorEffect(treatment, bandwidth, controls...)

	if err != nil {
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
			level, err := table.percentile(treatment, 0.75)

			if err == nil {
				interventionLevel = level
			}
		}

		if fitOK {
			upliftValue, err := nonLinear.counterfactualUplift(currentRow, treatment, interventionLevel)

			if err == nil {
				uplift = upliftValue
			}
		}

		if !fitOK {
			linear, err := table.fitLinearModel(predictors...)

			if err == nil {
				upliftValue, err := linear.counterfactualUplift(currentRow, treatment, interventionLevel)

				if err == nil {
					uplift = upliftValue
				}
			}
		}
	}

	invertedValue := 0.0

	if inverted {
		invertedValue = 1
	}

	state.MergeOutput("value", raw)
	state.MergeOutput("association", association)
	state.MergeOutput("intervention", intervention)
	state.MergeOutput("uplift", uplift)
	state.MergeOutput("contagion", contagion)
	state.MergeOutput("condition", condition)
	state.MergeOutput("inverted", invertedValue)
	return state.Read(p)
}

func (ladder *Ladder) Close() error {
	return nil
}
