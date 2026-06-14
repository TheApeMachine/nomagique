package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveMomentum(testingTB *testing.T) {
	Convey("Given ObserveMomentum", testingTB, func() {
		byFunction := MomentumState{}
		byMethod := MomentumState{}

		Convey("It should match method observation", func() {
			So(ObserveMomentum(&byFunction, 3), ShouldEqual, byMethod.Observe(3))
		})
	})
}

