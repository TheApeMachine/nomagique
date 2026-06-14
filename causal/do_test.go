package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNodeTable_DoExpectation(testingTB *testing.T) {
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

		expectation, expectErr := table.DoExpectation(2, 20, 0, 1)

		Convey("It should return a finite interventional expectation", func() {
			So(expectErr, ShouldBeNil)
			So(expectation, ShouldNotEqual, 0)
		})
	})
}

func BenchmarkNodeTable_DoExpectation(testingTB *testing.B) {
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

	if err != nil {
		testingTB.Fatal(err)
	}

	for testingTB.Loop() {
		_, _ = table.DoExpectation(2, 20, 0, 1)
	}
}
