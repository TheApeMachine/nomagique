package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveBeta(testingTB *testing.T) {
	Convey("Given a fresh Beta state", testingTB, func() {
		state := BetaState{}

		Convey("When bootstrapping a unit outcome", func() {
			mean := ObserveBeta(&state, 1)

			Convey("It should initialize posterior and return mean", func() {
				So(state.Ready, ShouldBeTrue)
				So(state.Alpha, ShouldAlmostEqual, 2, 1e-12)
				So(state.Beta, ShouldAlmostEqual, 1, 1e-12)
				So(mean, ShouldAlmostEqual, 2.0/3.0, 1e-12)
			})
		})
	})

	Convey("Given a ready Beta state", testingTB, func() {
		state := BetaState{}
		_ = ObserveBeta(&state, 0)
		firstMean := ObserveBeta(&state, 1)

		Convey("It should update posterior on subsequent outcomes", func() {
			So(firstMean, ShouldBeGreaterThan, 0)
			So(state.Alpha, ShouldBeGreaterThan, 1)
		})
	})
}

func TestObserveBetaPair(testingTB *testing.T) {
	Convey("Given a fresh Beta pair state", testingTB, func() {
		state := BetaState{}
		mean := ObserveBetaPair(&state, 10, 15)

		Convey("It should treat actual >= predicted as success", func() {
			So(state.Ready, ShouldBeTrue)
			So(state.Alpha, ShouldAlmostEqual, 2, 1e-12)
			So(mean, ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given a miss on first pair", testingTB, func() {
		state := BetaState{}
		mean := ObserveBetaPair(&state, 10, 5)

		Convey("It should treat actual < predicted as failure", func() {
			So(state.Beta, ShouldAlmostEqual, 2, 1e-12)
			So(mean, ShouldBeLessThan, 0.5)
		})
	})

	Convey("Given repeated pair observations", testingTB, func() {
		state := BetaState{}
		_ = ObserveBetaPair(&state, 10, 10)
		mean := ObserveBetaPair(&state, 10, 20)

		Convey("It should raise posterior after wins", func() {
			So(mean, ShouldBeGreaterThan, 0.5)
		})
	})
}
