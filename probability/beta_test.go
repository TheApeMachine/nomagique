package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBetaState_Observe(testingTB *testing.T) {
	Convey("Given a fresh Beta state", testingTB, func() {
		state := BetaState{}

		Convey("When bootstrapping a unit outcome", func() {
			value := state.Observe(1)

			Convey("It should initialize posterior and return mean", func() {
				So(state.Ready, ShouldBeTrue)
				So(state.Alpha, ShouldAlmostEqual, 2, 1e-12)
				So(state.Beta, ShouldAlmostEqual, 1, 1e-12)
				So(value, ShouldAlmostEqual, 2.0/3.0, 1e-12)
			})
		})
	})

	Convey("Given a ready Beta state", testingTB, func() {
		state := BetaState{}
		_ = state.Observe(0)
		firstMean := state.Observe(1)

		Convey("It should update posterior on subsequent outcomes", func() {
			So(firstMean, ShouldBeGreaterThan, 0)
			So(state.Alpha, ShouldBeGreaterThan, 1)
		})
	})
}

func TestBetaState_ObservePair(testingTB *testing.T) {
	Convey("Given a fresh Beta pair state", testingTB, func() {
		state := BetaState{}
		mean := state.ObservePair(10, 15)

		Convey("It should treat actual >= predicted as success", func() {
			So(state.Ready, ShouldBeTrue)
			So(state.Alpha, ShouldAlmostEqual, 2, 1e-12)
			So(mean, ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given a miss on first pair", testingTB, func() {
		state := BetaState{}
		mean := state.ObservePair(10, 5)

		Convey("It should treat actual < predicted as failure", func() {
			So(state.Beta, ShouldAlmostEqual, 2, 1e-12)
			So(mean, ShouldBeLessThan, 0.5)
		})
	})

	Convey("Given repeated pair observations", testingTB, func() {
		state := BetaState{}
		_ = state.ObservePair(10, 10)
		value := state.ObservePair(10, 20)

		Convey("It should raise posterior after wins", func() {
			So(value, ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given Beta pair history", testingTB, func() {
		state := BetaState{}
		_ = state.ObservePair(10, 10)
		value := state.ObservePair(10, 15)

		Convey("It should raise hit probability after wins", func() {
			So(value, ShouldBeGreaterThan, 0.5)
		})
	})
}

func TestBetaState_ObserveSamples(testingTB *testing.T) {
	Convey("Given outcomes", testingTB, func() {
		state := BetaState{}
		outcomes := []float64{1, 0, 1}
		out := make([]float64, len(outcomes))

		Convey("When observing in batch", func() {
			state.ObserveSamples(outcomes, out)

			Convey("It should match sequential observation", func() {
				expect := BetaState{}
				for index, outcome := range outcomes {
					So(out[index], ShouldEqual, expect.Observe(outcome))
				}
			})
		})
	})
}

func BenchmarkBetaState_Observe(testingTB *testing.B) {
	state := BetaState{}
	_ = state.Observe(1)

	for testingTB.Loop() {
		_ = state.Observe(0)
	}
}

func BenchmarkBetaState_ObservePair(testingTB *testing.B) {
	state := BetaState{}
	_ = state.ObservePair(10, 10)

	for testingTB.Loop() {
		_ = state.ObservePair(10, 11)
	}
}
