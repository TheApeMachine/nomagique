package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestEMA(testingTB *testing.T) {
	Convey("Given EMA constructor", testingTB, func() {
		exponential := EMA()

		Convey("It should return a usable dynamic", func() {
			So(exponential, ShouldNotBeNil)
		})
	})
}

func TestExponential_Observe(testingTB *testing.T) {
	Convey("Given a fresh exponential EMA", testingTB, func() {
		exponential := EMA()

		Convey("When bootstrapping with the first out", func() {
			value := exponential.Observe(core.Float64(10))

			Convey("It should adopt the out without error", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})

	Convey("Given an exponential fed a volatile series", testingTB, func() {
		exponential := EMA()
		series := []float64{10, 5, 20, 20, 15}
		var value core.Float64

		for _, out := range series {
			value = exponential.Observe(core.Float64(out))
		}

		Convey("It should derive a smoothed value inside the observed range", func() {
			So(float64(value), ShouldBeBetween, 5.0, 20.0)
		})
	})

	Convey("Given an exponential with a collapsed range", testingTB, func() {
		exponential := EMA()
		exponential.Observe(core.Float64(8))
		value := exponential.Observe(core.Float64(8))

		Convey("It should keep the dynamically implied value", func() {
			So(value, ShouldEqual, 8)
		})
	})

	Convey("Given an exponential extending its minimum", testingTB, func() {
		exponential := EMA()
		exponential.Observe(core.Float64(20))
		value := exponential.Observe(core.Float64(10))

		Convey("It should track below the prior minimum", func() {
			So(value, ShouldBeLessThan, 20)
		})
	})

	Convey("Given an exponential extending its maximum", testingTB, func() {
		exponential := EMA()
		exponential.Observe(core.Float64(5))
		value := exponential.Observe(core.Float64(25))

		Convey("It should track above the prior maximum", func() {
			So(value, ShouldBeGreaterThan, 5)
		})
	})

	Convey("Given exponential with prior out and work samples", testingTB, func() {
		exponential := EMA()
		exponential.Observe(core.Float64(5))

		Convey("When observing with out and raw work", func() {
			value := exponential.Observe(core.Float64(0), core.Float64(10))

			Convey("It should blend using out plus work", func() {
				So(value, ShouldBeGreaterThan, 0)
			})
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		exponential := EMA()

		Convey("When observing", func() {
			value := exponential.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestExponential_Reset(testingTB *testing.T) {
	Convey("Given an exponential with state", testingTB, func() {
		exponential := EMA()
		exponential.Observe(core.Float64(4))
		_ = exponential.Observe(core.Float64(6))

		Convey("When reset", func() {
			err := exponential.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(exponential.state.Value, ShouldEqual, 0)
				So(exponential.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func TestExponential_ObserveSamples(testingTB *testing.T) {
	Convey("Given an exponential", testingTB, func() {
		exponential := EMA()
		samples := []float64{1, 2, 3}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			exponential.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(len(out), ShouldEqual, 3)
			})
		})
	})
}

func BenchmarkExponential_Observe(testingTB *testing.B) {
	exponential := EMA()
	out := core.Float64(1)

	for testingTB.Loop() {
		exponential.Observe(out)
		out += 0.01
	}
}

func BenchmarkExponential_ObserveSamples(testingTB *testing.B) {
	exponential := EMA()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		exponential.state.Reset()
		exponential.ObserveSamples(samples, out)
	}
}
