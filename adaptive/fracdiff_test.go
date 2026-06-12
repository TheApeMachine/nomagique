package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestFracDiff(testingTB *testing.T) {
	Convey("Given FracDiff constructor", testingTB, func() {
		fracDiff := FracDiff()

		Convey("It should return a usable dynamic", func() {
			So(fracDiff, ShouldNotBeNil)
		})
	})
}

func TestFracDiff_Observe(testingTB *testing.T) {
	Convey("Given a fresh fractional differencing filter", testingTB, func() {
		fracDiff := FracDiff()

		Convey("When bootstrapping", func() {
			value := fracDiff.Observe(core.Float64(10))

			Convey("It should return the sample without error", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})

	Convey("Given fractional differencing history", testingTB, func() {
		fracDiff := FracDiff()
		fracDiff.Observe(core.Float64(10))
		value := fracDiff.Observe(core.Float64(15))

		Convey("It should emit a filtered value", func() {
			So(float64(value), ShouldNotEqual, 0)
		})
	})

	Convey("Given pipeline work samples", testingTB, func() {
		fractional := FracDiff()
		fractional.Observe(core.Float64(10))
		value := fractional.Observe(core.Float64(0), core.Float64(15))

		Convey("It should filter using the work sample", func() {
			So(float64(value), ShouldNotEqual, 0)
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		fracDiff := FracDiff()

		Convey("When observing", func() {
			value := fracDiff.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestFracDiff_ObserveSample(testingTB *testing.T) {
	Convey("Given fractional differencing", testingTB, func() {
		fracDiff := FracDiff()

		Convey("When observing a raw sample", func() {
			value := fracDiff.ObserveSample(7)

			Convey("It should return the sample on bootstrap", func() {
				So(value, ShouldEqual, 7)
			})
		})
	})
}

func TestFracDiff_ObserveSamples(testingTB *testing.T) {
	Convey("Given fractional differencing", testingTB, func() {
		fracDiff := FracDiff()
		samples := []float64{10, 12, 11}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			fracDiff.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(out[0], ShouldEqual, 10)
			})
		})
	})
}

func TestFracDiff_Reset(testingTB *testing.T) {
	Convey("Given fractional differencing with state", testingTB, func() {
		fracDiff := FracDiff()
		fracDiff.Observe(core.Float64(3))

		Convey("When reset", func() {
			err := fracDiff.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(fracDiff.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkFracDiff_Observe(testingTB *testing.B) {
	fracDiff := FracDiff()
	fracDiff.Observe(core.Float64(10))

	for testingTB.Loop() {
		fracDiff.Observe(core.Float64(10.01))
	}
}

func BenchmarkFracDiff_ObserveSamples(testingTB *testing.B) {
	fracDiff := FracDiff()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		fracDiff.state.Reset()
		fracDiff.ObserveSamples(samples, out)
	}
}
