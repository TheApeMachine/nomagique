package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestAccumulator(testingTB *testing.T) {
	Convey("Given Accumulator constructor", testingTB, func() {
		accumulator := Accumulator()

		Convey("It should return a usable dynamic", func() {
			So(accumulator, ShouldNotBeNil)
		})
	})
}

func TestAccumulator_Observe(testingTB *testing.T) {
	Convey("Given a fresh accumulator", testingTB, func() {
		accumulator := Accumulator()

		Convey("When charging", func() {
			value := accumulator.Observe(core.Float64(10))

			Convey("It should integrate without error", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})

	Convey("Given an accumulator with history", testingTB, func() {
		accumulator := Accumulator()
		accumulator.Observe(core.Float64(10))

		Convey("When draining", func() {
			value := accumulator.Observe(core.Float64(-4))

			Convey("It should subtract strength", func() {
				So(value, ShouldEqual, 6)
			})
		})

		Convey("When neutral", func() {
			value := accumulator.Observe(core.Float64(0))

			Convey("It should hold level", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})

	Convey("Given pipeline work samples", testingTB, func() {
		integrator := Accumulator()
		integrator.Observe(core.Float64(2), core.Float64(3))
		value := integrator.Observe(core.Float64(0), core.Float64(4))

		Convey("It should integrate through work", func() {
			So(value, ShouldEqual, 9)
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		accumulator := Accumulator()

		Convey("When observing", func() {
			value := accumulator.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestAccumulator_ObserveSample(testingTB *testing.T) {
	Convey("Given an accumulator", testingTB, func() {
		accumulator := Accumulator()

		Convey("When observing a raw sample", func() {
			value := accumulator.ObserveSample(7)

			Convey("It should return the integrated level", func() {
				So(value, ShouldEqual, 7)
			})
		})
	})
}

func TestAccumulator_ObserveSamples(testingTB *testing.T) {
	Convey("Given an accumulator", testingTB, func() {
		accumulator := Accumulator()
		samples := []float64{2, -1, 3}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			accumulator.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(out[2], ShouldEqual, 4)
			})
		})
	})
}

func TestAccumulator_Reset(testingTB *testing.T) {
	Convey("Given an accumulator with state", testingTB, func() {
		accumulator := Accumulator()
		accumulator.Observe(core.Float64(3))

		Convey("When reset", func() {
			err := accumulator.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(accumulator.state.Level, ShouldEqual, 0)
			})
		})
	})
}

func BenchmarkAccumulator_Observe(testingTB *testing.B) {
	accumulator := Accumulator()

	for testingTB.Loop() {
		accumulator.Observe(core.Float64(1))
	}
}

func BenchmarkAccumulator_ObserveSamples(testingTB *testing.B) {
	accumulator := Accumulator()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		accumulator.state.Reset()
		accumulator.ObserveSamples(samples, out)
	}
}
