package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEmaBatchScratch(testingTB *testing.T) {
	Convey("Given an EMA state", testingTB, func() {
		state := EMAState{}

		scratch := emaBatchScratch(&state, 4)

		Convey("It should allocate scratch for batch slots", func() {
			So(len(scratch), ShouldEqual, 4*batchScratchSlots)
		})
	})
}

func TestDeltaBatchScratch(testingTB *testing.T) {
	Convey("Given a delta state", testingTB, func() {
		state := DeltaState{}

		scratch := deltaBatchScratch(&state, 4)

		Convey("It should allocate paired scratch", func() {
			So(len(scratch), ShouldEqual, 8)
		})
	})
}
