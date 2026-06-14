package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveVariance(testingTB *testing.T) {
	Convey("Given ObserveVariance", testingTB, func() {
		byFunction := VarianceState{}
		byMethod := VarianceState{}

		Convey("It should match method observation", func() {
			So(ObserveVariance(&byFunction, 3), ShouldEqual, byMethod.Observe(3))
		})
	})
}

