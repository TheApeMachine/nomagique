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

func TestNodeTable_kernelBackdoorEffect(testingTB *testing.T) {
	Convey("Given a confounded table with a control", testingTB, func() {
		rows := [][]float64{
			{0, 0, 0},
			{1, 1, 2},
			{2, 2, 4},
			{3, 3, 6},
			{4, 4, 8},
			{5, 5, 10},
			{6, 6, 12},
			{7, 7, 14},
			{8, 8, 16},
			{9, 9, 18},
			{10, 10, 20},
			{11, 11, 22},
		}
		table, tableErr := newNodeTable(rows, 2, 12)

		So(tableErr, ShouldBeNil)

		linearEffect, linearErr := table.backdoorEffect(1, 0)
		kernelEffect, kernelErr := table.kernelBackdoorEffect(1, 0.5, 0)

		So(linearErr, ShouldBeNil)
		So(kernelErr, ShouldBeNil)

		Convey("It should align the kernel path with residualized backdoor adjustment", func() {
			So(kernelEffect, ShouldAlmostEqual, linearEffect, 1e-6)
		})
	})
}
