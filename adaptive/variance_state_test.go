package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestVarianceState_Observe(testingTB *testing.T) {
	Convey("Given a fresh variance state", testingTB, func() {
		state := VarianceState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given variance history", testingTB, func() {
		state := VarianceState{}
		_ = state.Observe(0)

		Convey("When observing a step", func() {
			value := state.Observe(10)

			Convey("It should derive positive variance", func() {
				So(value, ShouldBeGreaterThan, 0)
			})
		})
	})
}

func TestVarianceState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := VarianceState{}
		samples := []float64{0, 10}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := VarianceState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func TestVarianceState_Reset(testingTB *testing.T) {
	Convey("Given variance state", testingTB, func() {
		state := VarianceState{}
		_ = state.Observe(3)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkVarianceState_Observe(testingTB *testing.B) {
	state := VarianceState{}
	_ = state.Observe(1)

	for testingTB.Loop() {
		_ = state.Observe(1.01)
	}
}

func BenchmarkVarianceState_ObserveSamples(testingTB *testing.B) {
	state := VarianceState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}
