package kernel

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveEMAReady(testingTB *testing.T) {
	Convey("Given a ready EMA state", testingTB, func() {
		state := EMAState{}
		_ = ObserveEMA(&state, 5)

		Convey("When observing through the ready path", func() {
			readyState := state
			expectState := state
			byReady := observeEMAReady(&readyState, 7)
			expect := ObserveEMA(&expectState, 7)

			Convey("It should match ObserveEMA", func() {
				So(byReady, ShouldEqual, expect)
			})
		})
	})
}

func TestObserveDeltaReady(testingTB *testing.T) {
	Convey("Given a ready delta state", testingTB, func() {
		state := DeltaState{}
		_ = ObserveDelta(&state, 5)

		Convey("When observing through the ready path", func() {
			readyState := state
			expectState := state
			byReady := observeDeltaReady(&readyState, 7)
			expect := ObserveDelta(&expectState, 7)

			Convey("It should match ObserveDelta", func() {
				So(byReady, ShouldEqual, expect)
			})
		})
	})
}

func BenchmarkObserveEMAReady(testingTB *testing.B) {
	state := EMAState{}
	_ = ObserveEMA(&state, 0)
	sample := 1.0

	for testingTB.Loop() {
		sample = observeEMAReady(&state, sample+0.01)
	}
}

func BenchmarkObserveDeltaReady(testingTB *testing.B) {
	state := DeltaState{}
	_ = ObserveDelta(&state, 0)
	sample := 1.0

	for testingTB.Loop() {
		sample = observeDeltaReady(&state, sample+0.01)
	}
}
