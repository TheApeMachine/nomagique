package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gonum.org/v1/gonum/stat"
)

func TestNodeTable_Percentile(testingTB *testing.T) {
	Convey("Given a node history", testingTB, func() {
		rows := [][]float64{
			{0, 1, 2, 3},
			{4, 5, 6, 7},
			{8, 9, 10, 11},
			{12, 13, 14, 15},
		}

		table, err := NewNodeTable(rows, 3, 4)

		So(err, ShouldBeNil)

		value, valueErr := table.Percentile(0, 0.75)
		sorted := []float64{0, 4, 8, 12}
		expected := stat.Quantile(0.75, stat.LinInterp, sorted, nil)

		Convey("It should match gonum linear quantile interpolation", func() {
			So(valueErr, ShouldBeNil)
			So(value, ShouldEqual, expected)
		})
	})
}
