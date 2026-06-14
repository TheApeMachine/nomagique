package probability

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

	Convey("Given zero predicted in work pair", testingTB, func() {
		_, _, err := parsePredictedActual(
			core.Float64(0),
			[]core.Float64{0, 10},
		)

		Convey("It should return ErrZeroPredicted", func() {
			So(err, ShouldEqual, core.ErrZeroPredicted)
		})
	})
}

func TestParseBernoulliOutcome(testingTB *testing.T) {
	Convey("Given predicted and actual in work", testingTB, func() {
		outcome, err := parseBernoulliOutcome(
			core.Float64(0),
			[]core.Float64{10, 12},
		)

		Convey("It should emit success when actual meets predicted", func() {
			So(err, ShouldBeNil)
			So(outcome, ShouldEqual, 1)
		})
	})

	Convey("Given a raw probability in out", testingTB, func() {
		outcome, err := parseBernoulliOutcome(core.Float64(0.75), nil)

		Convey("It should pass through valid outcomes", func() {
			So(err, ShouldBeNil)
			So(outcome, ShouldEqual, 0.75)
		})
	})

	Convey("Given an invalid raw outcome", testingTB, func() {
		_, err := parseBernoulliOutcome(core.Float64(1.5), nil)

		Convey("It should return ErrInvalidOutcome", func() {
			So(err, ShouldEqual, core.ErrInvalidOutcome)
		})
	})
}
