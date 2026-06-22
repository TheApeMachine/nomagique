package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCompetitionMargin(testingTB *testing.T) {
	Convey("Given positive excess and span", testingTB, func() {
		margin, err := CompetitionMargin(1, 1)

		Convey("It should return a value in (0, 1)", func() {
			So(err, ShouldBeNil)
			So(margin, ShouldEqual, 0.5)
		})
	})

	Convey("Given non-positive excess or span", testingTB, func() {
		margin, err := CompetitionMargin(0, 1)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
			So(margin, ShouldEqual, 0)
		})
	})
}

func TestMagnitudeMargin(testingTB *testing.T) {
	Convey("Given a positive magnitude", testingTB, func() {
		margin, err := MagnitudeMargin(1)

		Convey("It should map into (0, 1)", func() {
			So(err, ShouldBeNil)
			So(margin, ShouldEqual, 0.5)
		})
	})

	Convey("Given a non-positive magnitude", testingTB, func() {
		margin, err := MagnitudeMargin(0)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
			So(margin, ShouldEqual, 0)
		})
	})
}
