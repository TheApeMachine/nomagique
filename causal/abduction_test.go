package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLinearModel_AbductiveCounterfactual(testingTB *testing.T) {
	Convey("Given a fitted linear structural model", testingTB, func() {
		rows := make([][]float64, 16)

		for index := range rows {
			rows[index] = []float64{
				float64(index),
				float64(index) * 0.5,
				float64(index) * 2,
				float64(index) * 0.25,
			}
		}

		table, err := NewNodeTable(rows, 3, 12)

		So(err, ShouldBeNil)

		model, modelErr := table.LinearModel(0, 1, 2)

		So(modelErr, ShouldBeNil)

		currentRow := rows[len(rows)-1]
		noise, noiseErr := model.AbductNoise(currentRow, currentRow[3])

		So(noiseErr, ShouldBeNil)

		counterfactual, cfErr := model.StructuralCounterfactual(currentRow, 2, 20, noise)

		So(cfErr, ShouldBeNil)

		Convey("It should preserve the abducted level at the observed treatment", func() {
			restored, restoreErr := model.StructuralCounterfactual(
				currentRow, 2, currentRow[2], noise,
			)

			So(restoreErr, ShouldBeNil)
			So(restored, ShouldAlmostEqual, currentRow[3], 1e-9)
		})

		Convey("It should move the counterfactual when treatment changes", func() {
			So(counterfactual, ShouldNotAlmostEqual, currentRow[3], 1e-9)
		})
	})
}

func TestAbductiveCounterfactual(testingTB *testing.T) {
	Convey("Given a fitted nonlinear model", testingTB, func() {
		rows := make([][]float64, 16)

		for index := range rows {
			rows[index] = []float64{
				float64(index) * 0.1,
				float64(index) * 0.2,
				float64(index) * 0.3,
				float64(index) * 0.05,
			}
		}

		table, err := NewNodeTable(rows, 3, 12)

		So(err, ShouldBeNil)

		model, fitOK := FitNonLinearTable(table, []int{0, 1, 2})

		So(fitOK, ShouldBeTrue)

		currentRow := rows[len(rows)-1]
		uplift, counterfactual, noise, cfErr := AbductiveCounterfactual(
			model, currentRow, 3, 2, 2.0,
		)

		Convey("It should return a finite abductive counterfactual read", func() {
			So(cfErr, ShouldBeNil)
			So(noise, ShouldNotEqual, 0)
			So(counterfactual, ShouldAlmostEqual, currentRow[3]+uplift, 1e-9)
		})
	})
}

func BenchmarkAbductiveCounterfactual(testingTB *testing.B) {
	rows := make([][]float64, 16)

	for index := range rows {
		rows[index] = []float64{
			float64(index) * 0.1,
			float64(index) * 0.2,
			float64(index) * 0.3,
			float64(index) * 0.05,
		}
	}

	table, err := NewNodeTable(rows, 3, 12)

	if err != nil {
		testingTB.Fatal(err)
	}

	model, fitOK := FitNonLinearTable(table, []int{0, 1, 2})

	if !fitOK {
		testingTB.Fatal("nonlinear fit failed")
	}

	currentRow := rows[len(rows)-1]

	for testingTB.Loop() {
		_, _, _, _ = AbductiveCounterfactual(model, currentRow, 3, 2, 2.0)
	}
}
