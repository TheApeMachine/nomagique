package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestParsePredictedActual(testingTB *testing.T) {
	Convey("Given out plus one work sample", testingTB, func() {
		predicted, actual, err := parsePredictedActual(core.Float64(10), []core.Float64{12})

		Convey("It should treat out as predicted and work as actual", func() {
			So(err, ShouldBeNil)
			So(predicted, ShouldEqual, 10)
			So(actual, ShouldEqual, 12)
		})
	})

	Convey("Given two work samples", testingTB, func() {
		predicted, actual, err := parsePredictedActual(
			core.Float64(0),
			[]core.Float64{10, 15},
		)

		Convey("It should read predicted and actual from work", func() {
			So(err, ShouldBeNil)
			So(predicted, ShouldEqual, 10)
			So(actual, ShouldEqual, 15)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		_, _, err := parsePredictedActual(core.Float64(0), []core.Float64{10})

		Convey("It should return ErrZeroPredicted", func() {
			So(err, ShouldEqual, core.ErrZeroPredicted)
		})
	})

	Convey("Given empty work", testingTB, func() {
		_, _, err := parsePredictedActual(core.Float64(10), nil)

		Convey("It should return ErrEmptyInputs", func() {
			So(err, ShouldEqual, core.ErrEmptyInputs)
		})
	})
}
