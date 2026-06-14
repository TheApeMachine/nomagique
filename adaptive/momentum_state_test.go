package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMomentumState_Observe(testingTB *testing.T) {
	Convey("Given a fresh momentum state", testingTB, func() {
		state := MomentumState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(0)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given momentum history", testingTB, func() {
		state := MomentumState{}
		_ = state.Observe(0)

		Convey("When price rises", func() {
			value := state.Observe(10)

			Convey("It should return positive momentum", func() {
				So(value, ShouldEqual, 1)
			})
		})

		Convey("When price falls", func() {
			state := MomentumState{}
			_ = state.Observe(20)
			value := state.Observe(10)

			Convey("It should return negative momentum", func() {
				So(value, ShouldEqual, -1)
			})
		})
	})
}

func TestMomentumState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := MomentumState{}
		samples := []float64{0, 10}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := MomentumState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func TestMomentumState_Reset(testingTB *testing.T) {
	Convey("Given momentum state", testingTB, func() {
		state := MomentumState{}
		_ = state.Observe(3)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkMomentumState_Observe(testingTB *testing.B) {
	state := MomentumState{}
	_ = state.Observe(1)

	for testingTB.Loop() {
		_ = state.Observe(1.01)
	}
}

func BenchmarkMomentumState_ObserveSamples(testingTB *testing.B) {
	state := MomentumState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}
