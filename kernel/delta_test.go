package kernel

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDeltaState_Observe(testingTB *testing.T) {
	Convey("Given a fresh delta state", testingTB, func() {
		state := DeltaState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given delta history", testingTB, func() {
		state := DeltaState{}
		_ = state.Observe(0)
		value := state.Observe(10)

		Convey("It should return a unit normalized step", func() {
			So(value, ShouldEqual, 1)
		})
	})
}

func TestDeltaState_Reset(testingTB *testing.T) {
	Convey("Given delta state", testingTB, func() {
		state := DeltaState{}
		_ = state.Observe(3)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func TestDeltaState_Observe_extendsMinimum(testingTB *testing.T) {
	Convey("Given delta state", testingTB, func() {
		state := DeltaState{}
		_ = state.Observe(20)
		value := state.Observe(10)

		Convey("It should derive a positive normalized change", func() {
			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func TestDeltaState_Observe_extendsMaximum(testingTB *testing.T) {
	Convey("Given delta state", testingTB, func() {
		state := DeltaState{}
		_ = state.Observe(5)
		value := state.Observe(25)

		Convey("It should derive a positive normalized change", func() {
			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func TestDeltaState_Observe_collapsedRange(testingTB *testing.T) {
	Convey("Given delta state", testingTB, func() {
		state := DeltaState{}
		_ = state.Observe(5)
		value := state.Observe(5)

		Convey("It should return zero for zero span", func() {
			So(value, ShouldEqual, 0)
		})
	})
}

func TestDeltaState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := DeltaState{}
		samples := []float64{0, 10}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := DeltaState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func BenchmarkDeltaState_ObserveSamples(testingTB *testing.B) {
	state := DeltaState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index)
	}

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}
