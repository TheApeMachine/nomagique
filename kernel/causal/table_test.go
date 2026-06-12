package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNodeTable_BackdoorEffect(testingTB *testing.T) {
	Convey("Given a linear causal table", testingTB, func() {
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

		effect, effectErr := table.BackdoorEffect(2, 0, 1)

		Convey("It should estimate a finite backdoor effect", func() {
			So(effectErr, ShouldBeNil)
			So(effect, ShouldNotEqual, 0)
		})
	})
}

func TestNodeTable_KernelBackdoorEffect(testingTB *testing.T) {
	Convey("Given enough history rows", testingTB, func() {
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

		effect, effectErr := table.KernelBackdoorEffect(2, 0.35, 0, 1)

		Convey("It should return a finite kernel backdoor effect", func() {
			So(effectErr, ShouldBeNil)
			So(effect, ShouldBeGreaterThan, 0)
		})
	})
}
