package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveEMABootstrap(testingTB *testing.T) {
	Convey("Given a cold EMA state", testingTB, func() {
		state := EMAState{}

		Convey("When observing the first sample", func() {
			value := ObserveEMA(&state, 10)

			Convey("It should bootstrap to the sample", func() {
				So(value, ShouldEqual, 10)
				So(state.Ready, ShouldBeTrue)
			})
		})
	})
}

func TestObserveDeltaBootstrap(testingTB *testing.T) {
	Convey("Given a cold delta state", testingTB, func() {
		state := DeltaState{}

		Convey("When observing the first sample", func() {
			value := ObserveDelta(&state, 10)

			Convey("It should return zero delta", func() {
				So(value, ShouldEqual, 0)
				So(state.Ready, ShouldBeTrue)
			})
		})
	})
}

func TestAbsExact(testingTB *testing.T) {
	Convey("Given negative and positive values", testingTB, func() {
		Convey("It should return exact magnitudes", func() {
			So(absExact(-3.5), ShouldEqual, 3.5)
			So(absExact(2), ShouldEqual, 2)
		})
	})
}
