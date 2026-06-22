package causal

import (
	"errors"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: missing table rows",
			errors.New("causal: table rows missing"),
		))
	}

	target := int(datura.Peek[float64](ladder.artifact, "target"))
	minHistory := int(datura.Peek[float64](ladder.artifact, "minHistory"))

	if minHistory <= 0 {
		minHistory = len(rows)
	}

	if len(rows) < minHistory {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: insufficient history",
			errors.New("causal: insufficient table history"),
		))
	}

	table, err := newNodeTable(rows, target, minHistory)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: table construction failed",
			err,
		))
	}

	inverted := datura.Peek[float64](state, "output", "value") != 0
	contagion := datura.Peek[float64](state, "paired")
	condition := datura.Peek[float64](state, "output", "condition")
	treatment := int(datura.Peek[float64](ladder.artifact, "treatmentNormal"))
	controls := intSlice(datura.Peek[[]float64](ladder.artifact, "controlsNormal"))

	if inverted {
		treatment = int(datura.Peek[float64](ladder.artifact, "treatmentInverted"))
		controls = intSlice(datura.Peek[[]float64](ladder.artifact, "controlsInverted"))
	}

	bandwidth := datura.Peek[float64](ladder.artifact, "kernelBandwidth")

	if bandwidth <= 0 {
		bandwidth = deriveBandwidth(rows, int(datura.Peek[float64](ladder.artifact, "treatmentNormal")))
	}

	if bandwidth <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: kernel bandwidth is invalid",
			errors.New("causal: kernel bandwidth must be positive"),
		))
	}

	association, err := table.association(treatment)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: association failed",
			err,
		))
	}

	intervention, err := table.kernelBackdoorEffect(treatment, bandwidth, controls...)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: kernel backdoor failed",
			err,
		))
	}

	raw := intervention
	uplift := 0.0

	if intervention > 0 {
		predictors := append(append([]int(nil), controls...), treatment)
		currentRow := rows[len(rows)-1]
		interventionLevel := datura.Peek[float64](ladder.artifact, "interventionLevel")

		if interventionLevel <= 0 {
			percentile := datura.Peek[float64](ladder.artifact, "interventionPercentile")

			if percentile <= 0 {
				percentile = 1 - 1/float64(len(rows))
			}

			level, percentileErr := table.percentile(treatment, percentile)

			if percentileErr != nil {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"causal ladder: intervention level failed",
					percentileErr,
				))
			}

			interventionLevel = level
		}

		nonLinear, fitOK := fitNonLinearTable(table, predictors)

		if fitOK {
			upliftValue, upliftErr := nonLinear.counterfactualUplift(currentRow, treatment, interventionLevel)

			if upliftErr != nil {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"causal ladder: nonlinear uplift failed",
					upliftErr,
				))
			}

			uplift = upliftValue
		}

		if !fitOK {
			linear, linearErr := table.fitLinearModel(predictors...)

			if linearErr != nil {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"causal ladder: linear uplift fit failed",
					linearErr,
				))
			}

			upliftValue, upliftErr := linear.counterfactualUplift(currentRow, treatment, interventionLevel)

			if upliftErr != nil {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"causal ladder: linear uplift failed",
					upliftErr,
				))
			}

			uplift = upliftValue
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
	state.Merge("root", "output")
	state.Merge("inputs", []string{
		"value", "association", "intervention", "uplift", "contagion", "condition", "inverted",
	})
	return state.Read(p)
}

func (ladder *Ladder) Close() error {
	return nil
}
