package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestDelta(testingTB *testing.T) {
	Convey("Given Delta constructor", testingTB, func() {
		delta := Delta()

		Convey("It should return a usable dynamic", func() {
			So(delta, ShouldNotBeNil)
		})
	})
}

func TestNormalized_Observe(testingTB *testing.T) {
	Convey("Given a fresh normalized delta", testingTB, func() {
		delta := Delta()

		Convey("When bootstrapping with the first sample", func() {
			value := delta.Observe(core.Float64(10))

			Convey("It should return zero without error", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given a normalized delta with history", testingTB, func() {
		delta := Delta()
		delta.Observe(core.Float64(0))

		Convey("When observing a unit step within range", func() {
			value := delta.Observe(core.Float64(0), core.Float64(10))

			Convey("It should return a normalized delta of one", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})

	Convey("Given a normalized delta with a collapsed range", testingTB, func() {
		delta := Delta()
		delta.Observe(core.Float64(8))
		value := delta.Observe(core.Float64(8))

		Convey("It should return zero", func() {
			So(value, ShouldEqual, 0)
		})
	})

	Convey("Given a normalized delta extending its minimum", testingTB, func() {
		delta := Delta()
		delta.Observe(core.Float64(20))
		value := delta.Observe(core.Float64(10))

		Convey("It should derive a positive normalized change", func() {
			So(value, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a normalized delta extending its maximum", testingTB, func() {
		delta := Delta()
		delta.Observe(core.Float64(5))
		value := delta.Observe(core.Float64(25))

		Convey("It should derive a positive normalized change", func() {
			So(value, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		delta := Delta()

		Convey("When observing", func() {
			value := delta.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestNormalized_ObserveSamples(testingTB *testing.T) {
	Convey("Given a normalized delta", testingTB, func() {
		delta := Delta()
		samples := []float64{0, 10}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			delta.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(out[1], ShouldEqual, 1)
			})
		})
	})
}

func BenchmarkNormalized_ObserveSamples(testingTB *testing.B) {
	delta := Delta()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		delta.state.Reset()
		delta.ObserveSamples(samples, out)
	}
}

func TestNormalized_Reset(testingTB *testing.T) {
	Convey("Given a normalized delta with state", testingTB, func() {
		delta := Delta()
		delta.Observe(core.Float64(3))

		Convey("When reset", func() {
			err := delta.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(delta.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkNormalized_Observe(testingTB *testing.B) {
	delta := Delta()

	for testingTB.Loop() {
		delta.Observe(core.Float64(1), core.Float64(2))
	}
}
