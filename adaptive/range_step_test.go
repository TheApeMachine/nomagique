package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveRange(testingTB *testing.T) {
	Convey("Given ObserveRange", testingTB, func() {
		byFunction := RangeState{}
		byMethod := RangeState{}

		Convey("It should match method observation", func() {
			So(ObserveRange(&byFunction, 3), ShouldEqual, byMethod.Observe(3))
		})
	})
}

