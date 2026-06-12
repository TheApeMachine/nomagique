package geom

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPhaseVelocityState_Observe(testingTB *testing.T) {
	Convey("Given a fresh phase velocity state", testingTB, func() {
		state := PhaseVelocityState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(1.5)

			Convey("It should return zero velocity", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given phase velocity history", testingTB, func() {
		state := PhaseVelocityState{}
		_ = state.Observe(1)
		value := state.Observe(1.5)

		Convey("It should return the mean difference", func() {
			So(value, ShouldAlmostEqual, 0.5, 1e-12)
		})
	})
}

func TestPhaseVelocityState_ObserveSamples(testingTB *testing.T) {
	Convey("Given means", testingTB, func() {
		state := PhaseVelocityState{}
		means := []float64{1, 1.5, 2}
		out := make([]float64, len(means))

		Convey("When observing in batch", func() {
			state.ObserveSamples(means, out)

			Convey("It should match sequential observation", func() {
				expect := PhaseVelocityState{}
				for index, mean := range means {
					So(out[index], ShouldEqual, expect.Observe(mean))
				}
			})
		})
	})
}

func BenchmarkPhaseVelocityState_Observe(testingTB *testing.B) {
	state := PhaseVelocityState{}
	_ = state.Observe(1)

	for testingTB.Loop() {
		_ = state.Observe(1.5)
	}
}
