package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestRange(testingTB *testing.T) {
	Convey("Given Range constructor", testingTB, func() {
		extent := Range()

		Convey("It should return a usable dynamic", func() {
			So(extent, ShouldNotBeNil)
		})
	})
}

func TestExtent_Observe(testingTB *testing.T) {
	Convey("Given a fresh range dynamic", testingTB, func() {
		extent := Range()

		Convey("When bootstrapping", func() {
			value := extent.Observe(core.Float64(10))

			Convey("It should return zero without error", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given range history", testingTB, func() {
		extent := Range()
		extent.Observe(core.Float64(10))
		value := extent.Observe(core.Float64(25))

		Convey("It should return the span", func() {
			So(value, ShouldEqual, 15)
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		extent := Range()

		Convey("When observing", func() {
			value := extent.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestExtent_ObserveSample(testingTB *testing.T) {
	Convey("Given range", testingTB, func() {
		extent := Range()
		_ = extent.ObserveSample(10)

		Convey("When extending the maximum", func() {
			value := extent.ObserveSample(25)

			Convey("It should return span", func() {
				So(value, ShouldEqual, 15)
			})
		})
	})
}

func TestExtent_ObserveSamples(testingTB *testing.T) {
	Convey("Given range", testingTB, func() {
		extent := Range()
		samples := []float64{10, 25}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			extent.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(out[1], ShouldEqual, 15)
			})
		})
	})
}

func TestExtent_Reset(testingTB *testing.T) {
	Convey("Given range with state", testingTB, func() {
		extent := Range()
		extent.Observe(core.Float64(3))

		Convey("When reset", func() {
			err := extent.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(extent.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkRange_Observe(testingTB *testing.B) {
	extent := Range()
	extent.Observe(core.Float64(1))

	for testingTB.Loop() {
		extent.Observe(core.Float64(1.01))
	}
}

func BenchmarkRange_ObserveSamples(testingTB *testing.B) {
	extent := Range()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		extent.state.Reset()
		extent.ObserveSamples(samples, out)
	}
}
