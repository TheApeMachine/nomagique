package kernel

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveEMA(t *testing.T) {
	Convey("Given ObserveEMA", t, func() {
		state := EMAState{}

		Convey("It should match method observation", func() {
			So(ObserveEMA(&state, 3), ShouldEqual, state.Observe(3))
		})
	})
}

func TestObserveDelta(t *testing.T) {
	Convey("Given ObserveDelta", t, func() {
		state := DeltaState{}
		byFunc := ObserveDelta(&state, 10)
		byMethod := state.Observe(10)

		Convey("It should match method observation", func() {
			So(byFunc, ShouldEqual, byMethod)
		})
	})
}
