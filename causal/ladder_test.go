package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEvaluate_Intervention(testingTB *testing.T) {
	Convey("Given sufficient ladder history", testingTB, func() {
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

		outcome := Evaluate(
			table,
			rows[len(rows)-1],
			0,
			LadderConfig{
				TreatmentNormal: 2,
				ControlsNormal:  []int{0, 1},
				KernelBandwidth: 0.35,
				MinHistory:      12,
			},
			NewRegimeTracker(),
		)

		Convey("It should return an intervention read on the ladder", func() {
			So(outcome.Intervention, ShouldBeGreaterThan, 0)
			So(outcome.Reason, ShouldEqual, "intervention")
			So(outcome.Raw, ShouldEqual, outcome.Intervention)
		})
	})
}

func TestEvaluate_CounterfactualLike(testingTB *testing.T) {
	Convey("Given confounded association and intervention", testingTB, func() {
		rows := make([][]float64, 20)

		for index := range rows {
			confounder := float64(index)
			treatment := confounder*0.8 + float64(index)*0.05
			rows[index] = []float64{
				confounder,
				treatment * 0.4,
				treatment,
				treatment*1.5 + confounder*0.7,
			}
		}

		table, err := NewNodeTable(rows, 3, 12)

		So(err, ShouldBeNil)

		currentRow := rows[len(rows)-1]

		outcome := Evaluate(
			table,
			currentRow,
			0,
			LadderConfig{
				TreatmentNormal:   2,
				ControlsNormal:    []int{0, 1},
				KernelBandwidth:   0.5,
				ConfoundFraction:  0.05,
				InterventionLevel: currentRow[2] + 5,
				MinHistory:        12,
			},
			NewRegimeTracker(),
		)

		Convey("It should label strong confounding as counterfactual-like when uplift is positive", func() {
			if outcome.Uplift > 0 {
				So(outcome.Reason, ShouldEqual, "counterfactual_like")
				So(outcome.Raw, ShouldEqual, outcome.Intervention)
				return
			}

			So(outcome.Intervention, ShouldBeGreaterThan, 0)
			So(outcome.Reason, ShouldEqual, "intervention")
		})
	})
}

func TestEvaluate_InsufficientHistory(testingTB *testing.T) {
	Convey("Given history below MinHistory", testingTB, func() {
		rows := make([][]float64, 8)

		for index := range rows {
			rows[index] = []float64{1, 2, 3, 4}
		}

		table, err := NewNodeTable(rows, 3, 4)

		So(err, ShouldBeNil)

		outcome := Evaluate(
			table,
			rows[len(rows)-1],
			0,
			LadderConfig{
				TreatmentNormal: 2,
				ControlsNormal:  []int{0, 1},
				KernelBandwidth: 0.35,
				MinHistory:      12,
			},
			NewRegimeTracker(),
		)

		Convey("It should return a zero outcome", func() {
			So(outcome.Raw, ShouldEqual, 0)
			So(outcome.Reason, ShouldBeBlank)
		})
	})
}

func BenchmarkEvaluate(testingTB *testing.B) {
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

	config := LadderConfig{
		TreatmentNormal: 2,
		ControlsNormal:  []int{0, 1},
		KernelBandwidth: 0.35,
		MinHistory:      12,
	}

	for testingTB.Loop() {
		_ = Evaluate(table, rows[len(rows)-1], 0, config, NewRegimeTracker())
	}
}
