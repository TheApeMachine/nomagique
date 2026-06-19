package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLinearModel_counterfactualUplift(testingTB *testing.T) {
	Convey("Given a linear model fit on treatment and controls", testingTB, func() {
		rows := [][]float64{
			{0, 0, 0, 0},
			{1, 2, 4, 1},
			{2, 4, 8, 2},
			{3, 6, 12, 3},
		}
		table, tableErr := newNodeTable(rows, 3, 4)

		So(tableErr, ShouldBeNil)

		model, modelErr := table.fitLinearModel(0, 1, 2)

		So(modelErr, ShouldBeNil)

		uplift, upliftErr := model.counterfactualUplift(rows[len(rows)-1], 2, 20)

		So(upliftErr, ShouldBeNil)
		So(uplift, ShouldBeGreaterThan, 0)
	})
}
