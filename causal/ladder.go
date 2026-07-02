package causal

import (
	"errors"
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Ladder evaluates Judea Pearl's ladder of causation over tabular rows on the artifact.
The constructor artifact holds config; Write buffers inbound table wire on its payload.
*/
type Ladder struct {
	artifact     *datura.Artifact
	pendingFrame bool
	output       []byte
}

/*
NewLadder returns a ladder stage wired from config attributes on the artifact.
*/
func NewLadder(artifact *datura.Artifact) *Ladder {
	return &Ladder{
		artifact: artifact,
	}
}

func (ladder *Ladder) Read(p []byte) (int, error) {
	if len(ladder.output) > 0 {
		return ladder.readOutput(p)
	}

	if !ladder.pendingFrame {
		return 0, io.EOF
	}

	state := datura.Acquire("ladder-state", datura.APPJSON)
	frame := ladder.artifact.DecryptPayload()

	if len(frame) == 0 {
		state.Release()
		ladder.pendingFrame = false

		return 0, io.EOF
	}

	if _, err := state.Unpack(frame); err != nil {
		state.Release()
		ladder.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: state write failed",
			err,
		))
	}

	defer state.Release()

	fail := func(err error) (int, error) {
		ladder.pendingFrame = false

		return 0, err
	}

	rows, err := tableRows(state)

	if err != nil {
		return fail(errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: missing table rows",
			err,
		)))
	}

	target := int(datura.Peek[float64](ladder.artifact, "target"))
	minHistory := int(datura.Peek[float64](ladder.artifact, "minHistory"))

	if minHistory <= 0 {
		return fail(errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: minHistory required",
			nil,
		)))
	}

	if len(rows) < minHistory {
		return fail(errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: insufficient table rows",
			nil,
		)))
	}

	table, err := newNodeTable(rows, target, minHistory)

	if err != nil {
		return fail(errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: table construction failed",
			err,
		)))
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
		bandwidth, err = deriveBandwidth(rows, int(datura.Peek[float64](ladder.artifact, "treatmentNormal")))

		if err != nil {
			if errors.Is(err, io.EOF) {
				return fail(io.EOF)
			}

			return fail(errnie.Error(errnie.Err(
				errnie.Validation,
				"causal ladder: kernel bandwidth derivation failed",
				err,
			)))
		}
	}

	if bandwidth <= 0 {
		return fail(errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: kernel bandwidth required",
			nil,
		)))
	}

	association, err := table.association(treatment)

	if err != nil {
		if errors.Is(err, io.EOF) {
			return fail(io.EOF)
		}

		return fail(errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: association failed",
			err,
		)))
	}

	intervention, err := table.kernelBackdoorEffect(treatment, bandwidth, controls...)

	if err != nil {
		if errors.Is(err, io.EOF) {
			return fail(io.EOF)
		}

		return fail(errnie.Error(errnie.Err(
			errnie.Validation,
			"causal ladder: kernel backdoor failed",
			err,
		)))
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

			level, err := table.percentile(treatment, percentile)

			if err != nil {
				return fail(errnie.Error(errnie.Err(
					errnie.Validation,
					"causal ladder: intervention level failed",
					err,
				)))
			}

			interventionLevel = level
		}

		nonLinear, fitOK := fitNonLinearTable(table, predictors)

		if fitOK {
			uplift, err = nonLinear.counterfactualUplift(currentRow, treatment, interventionLevel)

			if err != nil {
				return fail(errnie.Error(errnie.Err(
					errnie.Validation,
					"causal ladder: nonlinear uplift failed",
					err,
				)))
			}
		}

		if !fitOK {
			linear, err := table.fitLinearModel(predictors...)

			if err != nil {
				return fail(errnie.Error(errnie.Err(
					errnie.Validation,
					"causal ladder: linear uplift fit failed",
					err,
				)))
			}

			uplift, err = linear.counterfactualUplift(currentRow, treatment, interventionLevel)

			if err != nil {
				return fail(errnie.Error(errnie.Err(
					errnie.Validation,
					"causal ladder: linear uplift failed",
					err,
				)))
			}
		}
	}

	invertedValue := 0.0

	if inverted {
		invertedValue = 1
	}

	interventionScore := intervention
	upliftScore := uplift

	if targetValues, targetErr := table.column(target); targetErr == nil {
		if scale := robustScale(targetValues); scale > 0 {
			interventionScore = intervention / scale
			upliftScore = uplift / scale
		}
	}

	state.MergeOutput("value", raw)
	state.MergeOutput("association", association)
	state.MergeOutput("intervention", intervention)
	state.MergeOutput("interventionScore", interventionScore)
	state.MergeOutput("uplift", uplift)
	state.MergeOutput("upliftScore", upliftScore)
	state.MergeOutput("contagion", contagion)
	state.MergeOutput("condition", condition)
	state.MergeOutput("inverted", invertedValue)
	state.Poke("output", "root")
	state.Poke([]string{
		"value", "association", "intervention", "interventionScore", "uplift", "upliftScore", "contagion", "condition", "inverted",
	}, "inputs")
	ladder.output = state.Pack()

	return ladder.readOutput(p)
}

func (ladder *Ladder) Write(p []byte) (int, error) {
	if len(p) == 0 {
		ladder.pendingFrame = false
		ladder.output = nil

		return 0, nil
	}

	ladder.artifact.WithPayload(p)
	ladder.pendingFrame = true
	ladder.output = nil

	return len(p), nil
}

func (ladder *Ladder) readOutput(p []byte) (int, error) {
	n := copy(p, ladder.output)

	if n < len(ladder.output) {
		return n, io.ErrShortBuffer
	}

	ladder.output = nil
	ladder.pendingFrame = false

	return n, io.EOF
}

func (ladder *Ladder) Close() error {
	return nil
}

func robustScale(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	center := median(values)
	deviations := make([]float64, 0, len(values))

	for _, value := range values {
		deviations = append(deviations, math.Abs(value-center))
	}

	scale := median(deviations)

	if scale > 0 && !math.IsNaN(scale) && !math.IsInf(scale, 0) {
		return scale
	}

	minValue := values[0]
	maxValue := values[0]

	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}

		if value > maxValue {
			maxValue = value
		}
	}

	scale = maxValue - minValue

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0
	}

	return scale
}
