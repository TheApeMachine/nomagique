package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestVariance(testingTB *testing.T) {
	Convey("Given Variance constructor", testingTB, func() {
		dispersion := Variance()

		Convey("It should return a usable dynamic", func() {
			So(dispersion, ShouldNotBeNil)
		})
	})
}

func TestDispersion_Observe(testingTB *testing.T) {
	Convey("Given a fresh variance dynamic", testingTB, func() {
		dispersion := Variance()

		Convey("When bootstrapping", func() {
			value := dispersion.Observe(core.Float64(10))

			Convey("It should return zero without error", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given variance history", testingTB, func() {
		dispersion := Variance()
		dispersion.Observe(core.Float64(0))
		value := dispersion.Observe(core.Float64(10))

		Convey("It should derive positive variance", func() {
			So(float64(value), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		dispersion := Variance()

		Convey("When observing", func() {
			value := dispersion.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestDispersion_ObserveSample(testingTB *testing.T) {
	Convey("Given variance", testingTB, func() {
		dispersion := Variance()
		_ = dispersion.ObserveSample(0)

		Convey("When observing a step", func() {
			value := dispersion.ObserveSample(10)

			Convey("It should return variance", func() {
				So(value, ShouldBeGreaterThan, 0)
			})
		})
	})
}

func TestDispersion_ObserveSamples(testingTB *testing.T) {
	Convey("Given variance", testingTB, func() {
		dispersion := Variance()
		samples := []float64{0, 10}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			dispersion.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(out[1], ShouldBeGreaterThan, 0)
			})
		})
	})
}

func TestDispersion_Reset(testingTB *testing.T) {
	Convey("Given variance with state", testingTB, func() {
		dispersion := Variance()
		dispersion.Observe(core.Float64(3))

		Convey("When reset", func() {
			err := dispersion.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(dispersion.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkVariance_Observe(testingTB *testing.B) {
	dispersion := Variance()
	dispersion.Observe(core.Float64(1))

	for testingTB.Loop() {
		dispersion.Observe(core.Float64(1.01))
	}
}

func BenchmarkVariance_ObserveSamples(testingTB *testing.B) {
	dispersion := Variance()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		dispersion.state.Reset()
		dispersion.ObserveSamples(samples, out)
	}
}
