package causal

import (
	"github.com/theapemachine/datura"
)

/*
Do estimates E[Y|do(X=level)] via the g-formula over observed covariates.
*/
type Do struct {
	artifact *datura.Artifact
}

/*
NewDo returns an interventional expectation stage reading config and table rows from the artifact.
*/
func NewDo() *Do {
	return &Do{
		artifact: datura.Acquire("do", datura.APPJSON),
	}
}

func (doStage *Do) Write(p []byte) (int, error) {
	return doStage.artifact.Write(p)
}

func (doStage *Do) Read(p []byte) (int, error) {
	rows, ok := tableRows(doStage.artifact)

	if !ok {
		doStage.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return doStage.artifact.Read(p)
	}

	target := int(datura.Peek[float64](doStage.artifact, "config", "target"))
	treatment := int(datura.Peek[float64](doStage.artifact, "config", "treatment"))
	level := datura.Peek[float64](doStage.artifact, "config", "level")
	controls := intSlice(datura.Peek[[]float64](doStage.artifact, "config", "controls"))
	minRows := int(datura.Peek[float64](doStage.artifact, "config", "minHistory"))

	if minRows <= 0 {
		minRows = 12
	}

	table, err := newNodeTable(rows, target, minRows)

	if err != nil {
		doStage.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return doStage.artifact.Read(p)
	}

	expectation, expectErr := doExpectation(table, treatment, level, controls...)

	if expectErr != nil {
		doStage.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return doStage.artifact.Read(p)
	}

	doStage.artifact.Poke(datura.Map[float64]{"value": expectation}, "output")

	return doStage.artifact.Read(p)
}

func (doStage *Do) Close() error {
	return nil
}

func doExpectation(
	nodeTable nodeTable,
	treatment int,
	level float64,
	controls ...int,
) (float64, error) {
	predictors := append(append([]int(nil), controls...), treatment)
	model, err := nodeTable.fitLinearModel(predictors...)

	if err != nil {
		return 0, err
	}

	total := 0.0

	for _, row := range nodeTable.rows {
		prediction, predErr := model.predict(row, treatment, level)

		if predErr != nil {
			return 0, predErr
		}

		total += prediction
	}

	return total / float64(len(nodeTable.rows)), nil
}
