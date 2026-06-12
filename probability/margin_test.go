package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCompetitionMargin(testingTB *testing.T) {
	Convey("Given positive excess and span", testingTB, func() {
		margin := CompetitionMargin(1, 1)

		Convey("It should return a value in (0, 1)", func() {
			So(margin, ShouldEqual, 0.5)
		})
	})
}

func TestMagnitudeMargin(testingTB *testing.T) {
	Convey("Given a positive magnitude", testingTB, func() {
		margin := MagnitudeMargin(1)

		Convey("It should map into (0, 1)", func() {
			So(margin, ShouldEqual, 0.5)
		})
	})
}
