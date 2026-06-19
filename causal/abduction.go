package causal

import (
	"errors"

	"github.com/theapemachine/datura"
)

/*
Abduction runs abductive counterfactual inference from table.* and config on the artifact.
*/
type Abduction struct {
	artifact *datura.Artifact
}

/*
NewAbduction returns an abduction stage reading table rows and config from the artifact.
*/
func NewAbduction() *Abduction {
	return &Abduction{
		artifact: datura.Acquire("abduction", datura.APPJSON),
	}
}

func (abduction *Abduction) Write(p []byte) (int, error) {
	return abduction.artifact.Write(p)
}

func (abduction *Abduction) Read(p []byte) (int, error) {
	rows, ok := tableRows(abduction.artifact)

	if !ok {
		abduction.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return abduction.artifact.Read(p)
	}

	target := int(datura.Peek[float64](abduction.artifact, "config", "target"))
	treatment := int(datura.Peek[float64](abduction.artifact, "config", "treatment"))
	intervention := datura.Peek[float64](abduction.artifact, "config", "intervention")
	minHistory := int(datura.Peek[float64](abduction.artifact, "config", "minHistory"))
	features := intSlice(datura.Peek[[]float64](abduction.artifact, "config", "features"))
	linear := datura.Peek[float64](abduction.artifact, "config", "linear") > 0

	if minHistory <= 0 {
		minHistory = 12
	}

	table, err := newNodeTable(rows, target, minHistory)

	if err != nil {
		abduction.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return abduction.artifact.Read(p)
	}

	currentRow := rows[len(rows)-1]
	uplift, counterfactual, noise, cfErr := abductiveCounterfactual(
		table,
		features,
		linear,
		currentRow,
		target,
		treatment,
		intervention,
	)

	if cfErr != nil {
		abduction.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return abduction.artifact.Read(p)
	}

	abduction.artifact.Poke(datura.Map[float64]{
		"value":          uplift,
		"uplift":         uplift,
		"counterfactual": counterfactual,
		"noise":          noise,
	}, "output")

	return abduction.artifact.Read(p)
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
		model, modelErr := table.fitLinearModel(features...)

		if modelErr != nil {
			return 0, 0, 0, modelErr
		}

		noise, err = abductNoiseLinear(model, row, observedTarget)

		if err != nil {
			return 0, 0, 0, err
		}

		counterfactual, err = structuralCounterfactualLinear(model, row, treatment, intervention, noise)

		if err != nil {
			return 0, 0, 0, err
		}

		uplift, err = model.counterfactualUplift(row, treatment, intervention)

		if err != nil {
			return 0, 0, 0, err
		}

		return uplift, counterfactual, noise, nil
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
