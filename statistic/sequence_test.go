package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLinSpace(testingTB *testing.T) {
	Convey("Given endpoints and a count", testingTB, func() {
		values := LinSpace(0, 1, 3)

		Convey("It should return evenly spaced values", func() {
			So(len(values), ShouldEqual, 3)
			So(values[0], ShouldEqual, 0)
			So(values[2], ShouldEqual, 1)
		})
	})
}

func TestLogSpace(testingTB *testing.T) {
	Convey("Given positive endpoints", testingTB, func() {
		values := LogSpace(1, 10, 3)

		Convey("It should return logarithmically spaced values", func() {
			So(len(values), ShouldEqual, 3)
			So(values[0], ShouldEqual, 1)
			So(values[2], ShouldAlmostEqual, 10, 1e-9)
		})
	})
}

func TestQuartiles(testingTB *testing.T) {
	Convey("Given a sample", testingTB, func() {
		lower, upper := Quartiles([]float64{1, 2, 3, 4, 5, 6, 7, 8})

		Convey("It should return lower and upper quartiles", func() {
			So(lower, ShouldBeLessThan, upper)
		})
	})
}
