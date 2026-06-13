package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAccumulatorState_Observe(testingTB *testing.T) {
	Convey("Given a fresh accumulator", testingTB, func() {
		state := AccumulatorState{}

		Convey("When charging with a positive sample", func() {
			value := state.Observe(10)

			Convey("It should integrate the sample strength", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})

	Convey("Given an accumulator with level", testingTB, func() {
		state := AccumulatorState{}
		_ = state.Observe(10)

		Convey("When draining with a negative sample", func() {
			value := state.Observe(-3)

			Convey("It should subtract proportional strength", func() {
				So(value, ShouldEqual, 7)
			})
		})

		Convey("When receiving neutral signal", func() {
			value := state.Observe(0)

			Convey("It should hold the level", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})
}

func TestAccumulatorState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := AccumulatorState{}
		samples := []float64{5, -2, 0, 3}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := AccumulatorState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func TestAccumulatorState_Reset(testingTB *testing.T) {
	Convey("Given accumulator state", testingTB, func() {
		state := AccumulatorState{}
		_ = state.Observe(4)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear the level", func() {
				So(state.Level, ShouldEqual, 0)
			})
		})
	})
}

func TestObserveAccumulator(testingTB *testing.T) {
	Convey("Given ObserveAccumulator", testingTB, func() {
		byFunction := AccumulatorState{}
		byMethod := AccumulatorState{}

		Convey("It should match method observation", func() {
			So(ObserveAccumulator(&byFunction, 3), ShouldEqual, byMethod.Observe(3))
		})
	})
}

func BenchmarkAccumulatorState_Observe(testingTB *testing.B) {
	state := AccumulatorState{}
	_ = state.Observe(1)

	for testingTB.Loop() {
		_ = state.Observe(0.01)
	}
}

func BenchmarkAccumulatorState_ObserveSamples(testingTB *testing.B) {
	state := AccumulatorState{}
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for index := range samples {
		samples[index] = float64(index%7) - 3
	}

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(samples, out)
	}
}
