package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveCompression(testingTB *testing.T) {
	Convey("Given ObserveCompression", testingTB, func() {
		state := CompressionState{}

		Convey("It should match method observation", func() {
			So(ObserveCompression(&state, 8), ShouldEqual, state.Observe(8))
		})
	})
}

func TestObserveCompressionReady(testingTB *testing.T) {
	Convey("Given ObserveCompressionReady", testingTB, func() {
		state := CompressionState{Baseline: 20, Ready: true}

		Convey("It should match ready observation", func() {
			So(observeCompressionReady(&state, 10), ShouldEqual, state.Observe(10))
		})
	})
}

