package causal

import (
	"errors"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Abduction runs abductive counterfactual inference from table.* and config on the artifact.
The constructor artifact holds config; Write buffers inbound table wire on its payload.
*/
type Abduction struct {
	artifact *datura.Artifact
}

/*
NewAbduction returns an abduction stage wired from config attributes on the artifact.
*/
func NewAbduction(artifact *datura.Artifact) *Abduction {
	artifact.Inspect("causal", "abduction", "NewAbduction()")

	return &Abduction{
		artifact: artifact,
	}
}

func (abduction *Abduction) Write(p []byte) (int, error) {
	abduction.artifact.WithPayload(p)
	return len(p), nil
}

func (abduction *Abduction) Read(p []byte) (int, error) {
	state := datura.Acquire("abduction-state", datura.APPJSON)
	state.Inspect("causal", "abduction", "Read()", "p")

	if _, err := state.Write(abduction.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	rows, ok := tableRows(state)

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal abduction: missing table rows",
			errors.New("causal: table rows missing"),
		))
	}

	target := int(datura.Peek[float64](abduction.artifact, "target"))
	treatment := int(datura.Peek[float64](abduction.artifact, "treatment"))
	intervention := datura.Peek[float64](abduction.artifact, "intervention")
	minHistory := int(datura.Peek[float64](abduction.artifact, "minHistory"))
	features := intSlice(datura.Peek[[]float64](abduction.artifact, "features"))
	linear := datura.Peek[float64](abduction.artifact, "linear") > 0

	if minHistory <= 0 {
		minHistory = len(rows)
	}

	table, err := newNodeTable(rows, target, minHistory)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal abduction: table construction failed",
			err,
		))
	}

	currentRow := rows[len(rows)-1]
	uplift, counterfactual, noise, err := abductiveCounterfactual(
		table,
		features,
		linear,
		currentRow,
		target,
		treatment,
		intervention,
	)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal abduction: counterfactual failed",
			err,
		))
	}

	state.MergeOutput("value", uplift)
	state.MergeOutput("uplift", uplift)
	state.MergeOutput("counterfactual", counterfactual)
	state.MergeOutput("noise", noise)
	return state.Read(p)
}

func (abduction *Abduction) Close() error {
	return nil
}

func abductiveCounterfactual(
	table nodeTable,
	features []int,
	linear bool,
	row []float64,
	target int,
	treatment int,
	intervention float64,
) (uplift, counterfactual, noise float64, err error) {
	if target < 0 || target >= len(row) {
		return 0, 0, 0, errors.New("causal: abduction target outside row")
	}

	observedTarget := row[target]

	if linear {
		model, err := table.fitLinearModel(features...)

		if err != nil {
			return 0, 0, 0, err
		}

		noise, err = abductNoiseLinear(model, row, observedTarget)

		if err != nil {
			return 0, 0, 0, err
		}

		counterfactual, err = structuralCounterfactualLinear(model, row, treatment, intervention, noise)

		if err != nil {
			return 0, 0, 0, err
		}

		return counterfactual - observedTarget, counterfactual, noise, nil
	}

	model, fitOK := fitNonLinearTable(table, features)

	if !fitOK {
		return 0, 0, 0, errors.New("causal: nonlinear abduction fit failed")
	}

	noise, err = abductNoiseNonLinear(model, row, observedTarget)

	if err != nil {
		return 0, 0, 0, err
	}

	counterfactual, err = structuralCounterfactualNonLinear(model, row, treatment, intervention, noise)

	if err != nil {
		return 0, 0, 0, err
	}

	return counterfactual - observedTarget, counterfactual, noise, nil
}

func abductNoiseLinear(model linearModel, row []float64, observedTarget float64) (float64, error) {
	fitted, err := model.predict(row, -1, 0)

	if err != nil {
		return 0, err
	}

	return observedTarget - fitted, nil
}

func structuralCounterfactualLinear(
	model linearModel,
	row []float64,
	treatment int,
	intervention float64,
	noise float64,
) (float64, error) {
	structural, err := model.predict(row, treatment, intervention)

	if err != nil {
		return 0, err
	}

	return structural + noise, nil
}

func abductNoiseNonLinear(model nonLinearModel, row []float64, observedTarget float64) (float64, error) {
	fitted, err := model.predict(row, -1, 0)

	if err != nil {
		return 0, err
	}

	return observedTarget - fitted, nil
}

func structuralCounterfactualNonLinear(
	model nonLinearModel,
	row []float64,
	treatment int,
	intervention float64,
	noise float64,
) (float64, error) {
	structural, err := model.predict(row, treatment, intervention)

	if err != nil {
		return 0, err
	}

	return structural + noise, nil
}
