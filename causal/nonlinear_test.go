package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func buildNonLinearTable(testingTB *testing.T) NodeTable {
	testingTB.Helper()

	rows := make([][]float64, 20)

	for index := range rows {
		feature := float64(index)
		rows[index] = []float64{
			feature,
			feature * 0.5,
			feature*feature*0.01 + feature*0.3,
		}
	}

	table, err := NewNodeTable(rows, 2, 12)

	So(err, ShouldBeNil)

	return table
}

func TestFitNonLinearTable(testingTB *testing.T) {
	Convey("Given empty features", testingTB, func() {
		table := buildNonLinearTable(testingTB)
		model, ok := FitNonLinearTable(table, nil)

		Convey("It should refuse to fit", func() {
			So(ok, ShouldBeFalse)
			So(len(model.stumps), ShouldEqual, 0)
		})
	})

	Convey("Given valid feature indices", testingTB, func() {
		table := buildNonLinearTable(testingTB)
		model, ok := FitNonLinearTable(table, []int{0, 1})

		Convey("It should fit at least one stump", func() {
			So(ok, ShouldBeTrue)
			So(len(model.stumps), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given invalid feature index", testingTB, func() {
		table := buildNonLinearTable(testingTB)
		model, ok := FitNonLinearTable(table, []int{99})

		Convey("It should reject out-of-range nodes", func() {
			So(ok, ShouldBeFalse)
			So(len(model.stumps), ShouldEqual, 0)
		})
	})
}

func TestNonLinearModel_Predict(testingTB *testing.T) {
	Convey("Given a fitted stump ensemble", testingTB, func() {
		table := buildNonLinearTable(testingTB)
		model, ok := FitNonLinearTable(table, []int{0, 1})

		So(ok, ShouldBeTrue)

		row := table.rows[10]
		prediction, err := model.Predict(row, -1, 0)

		Convey("It should return a finite prediction", func() {
			So(err, ShouldBeNil)
			So(prediction, ShouldNotEqual, 0)
		})
	})
}

func TestNonLinearModel_CounterfactualUplift(testingTB *testing.T) {
	Convey("Given a fitted model and intervention", testingTB, func() {
		table := buildNonLinearTable(testingTB)
		model, ok := FitNonLinearTable(table, []int{0, 1})

		So(ok, ShouldBeTrue)

		row := table.rows[5]
		uplift, err := model.CounterfactualUplift(row, 0, 100)

		Convey("It should return counterfactual minus observed", func() {
			So(err, ShouldBeNil)

			observed, predictErr := model.Predict(row, -1, 0)
			counterfactual, counterErr := model.Predict(row, 0, 100)

			So(predictErr, ShouldBeNil)
			So(counterErr, ShouldBeNil)
			So(uplift, ShouldAlmostEqual, counterfactual-observed, 1e-12)
		})
	})
}

func TestNonLinearModel_PredictInvalidRow(testingTB *testing.T) {
	Convey("Given a stump referencing missing features", testingTB, func() {
		model := NonLinearModel{
			intercept: 1,
			stumps: []stumpSplit{{
				featureIndex: 0,
				threshold:    0,
				leftMean:     1,
				rightMean:    2,
			}},
		}

		_, err := model.Predict(nil, -1, 0)

		Convey("It should error on short rows", func() {
			So(err, ShouldNotBeNil)
		})
	})
}
