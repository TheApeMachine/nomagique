package causal

import (
	"github.com/theapemachine/datura"
)

/*
Do estimates E[Y|do(X=level)] via the g-formula over observed covariates.
The constructor artifact holds config; Write buffers inbound table wire on its payload.
*/
type Do struct {
	artifact *datura.Artifact
}

/*
NewDo returns an interventional expectation stage wired from config attributes on the artifact.
*/
func NewDo(artifact *datura.Artifact) *Do {
	artifact.Inspect("causal", "do", "NewDo()")

	return &Do{
		artifact: artifact,
	}
}

func (doStage *Do) Write(p []byte) (int, error) {
	doStage.artifact.WithPayload(p)
	return len(p), nil
}

func (doStage *Do) Read(p []byte) (int, error) {
	state := datura.Acquire("do-state", datura.APPJSON)
	state.Inspect("causal", "do", "Read()", "p")

	if _, err := state.Write(doStage.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	rows, ok := tableRows(state)

	if !ok {
		state.MergeOutput("value", 0.0)
		return state.Read(p)
	}

	target := int(datura.Peek[float64](doStage.artifact, "config", "target"))
	treatment := int(datura.Peek[float64](doStage.artifact, "config", "treatment"))
	level := datura.Peek[float64](doStage.artifact, "config", "level")
	controls := intSlice(datura.Peek[[]float64](doStage.artifact, "config", "controls"))
	minRows := int(datura.Peek[float64](doStage.artifact, "config", "minHistory"))

	if minRows <= 0 {
		minRows = len(rows)
	}

	table, err := newNodeTable(rows, target, minRows)

	if err != nil {
		state.MergeOutput("value", 0.0)
		return state.Read(p)
	}

	expectation, err := doExpectation(table, treatment, level, controls...)

	if err != nil {
		state.MergeOutput("value", 0.0)
		return state.Read(p)
	}

	state.MergeOutput("value", expectation)
	return state.Read(p)
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
		prediction, err := model.predict(row, treatment, level)

		if err != nil {
			return 0, err
		}

		total += prediction
	}

	return total / float64(len(nodeTable.rows)), nil
}
