package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestParseGrowthPair(testingTB *testing.T) {
	Convey("Given out plus one work sample", testingTB, func() {
		left, right, err := parseGrowthPair(core.Float64(2), []core.Float64{3})

		Convey("It should treat out as left and work as right", func() {
			So(err, ShouldBeNil)
			So(left, ShouldEqual, 2)
			So(right, ShouldEqual, 3)
		})
	})

	Convey("Given two work samples", testingTB, func() {
		left, right, err := parseGrowthPair(
			core.Float64(0),
			[]core.Float64{1.5, -0.5},
		)

		Convey("It should read both operands from work", func() {
			So(err, ShouldBeNil)
			So(left, ShouldEqual, 1.5)
			So(right, ShouldEqual, -0.5)
		})
	})

	Convey("Given empty work", testingTB, func() {
		_, _, err := parseGrowthPair(core.Float64(1), nil)

		Convey("It should return ErrEmptyInputs", func() {
			So(err, ShouldEqual, core.ErrEmptyInputs)
		})
	})
}
