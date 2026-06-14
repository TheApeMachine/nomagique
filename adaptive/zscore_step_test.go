package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveZScore(testingTB *testing.T) {
	Convey("Given ObserveZScore", testingTB, func() {
		byFunction := ZScoreState{}
		byMethod := ZScoreState{}

		Convey("It should match method observation", func() {
			So(
				ObserveZScore(&byFunction, 3, 0, false),
				ShouldEqual,
				byMethod.Observe(3),
			)
		})
	})
}

