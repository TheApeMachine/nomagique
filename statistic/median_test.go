package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMedianOf(t *testing.T) {
	Convey("Given unsorted values", t, func() {
		Convey("It should return the median", func() {
			value, ok := MedianOf([]float64{3, 1, 2})
			So(ok, ShouldBeTrue)
			So(value, ShouldEqual, 2)
		})
	})

	Convey("Given empty values", t, func() {
		Convey("It should reject empty input", func() {
			_, ok := MedianOf(nil)
			So(ok, ShouldBeFalse)
		})
	})

	Convey("Given non-finite values", t, func() {
		Convey("It should reject non-finite input", func() {
			_, ok := MedianOf([]float64{1, math.NaN(), 3})
			So(ok, ShouldBeFalse)
		})
	})
}
