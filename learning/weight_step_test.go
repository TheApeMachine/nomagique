package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveWeight(testingTB *testing.T) {
	Convey("Given ObserveWeight", testingTB, func() {
		byFunction := WeightState{}
		byMethod := WeightState{}

		Convey("It should match method observation", func() {
			So(
				ObserveWeight(&byFunction, 10, 10),
				ShouldEqual,
				byMethod.Observe(10, 10),
			)
		})
	})
}

