package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveAccumulator(testingTB *testing.T) {
	Convey("Given ObserveAccumulator", testingTB, func() {
		byFunction := AccumulatorState{}
		byMethod := AccumulatorState{}

		Convey("It should match method observation", func() {
			So(ObserveAccumulator(&byFunction, 3), ShouldEqual, byMethod.Observe(3))
		})
	})
}

