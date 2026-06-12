package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestZScore(testingTB *testing.T) {
	Convey("Given ZScore constructor", testingTB, func() {
		surprise := ZScore()

		Convey("It should return a usable dynamic", func() {
			So(surprise, ShouldNotBeNil)
		})
	})
}

func TestSurprise_Observe(testingTB *testing.T) {
	Convey("Given a fresh z-score dynamic", testingTB, func() {
		surprise := ZScore()

		Convey("When bootstrapping", func() {
			value := surprise.Observe(core.Float64(10))

			Convey("It should return zero without error", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given z-score with EMA anchor work", testingTB, func() {
		surprise := ZScore()
		surprise.Observe(core.Float64(0))
		value := surprise.Observe(core.Float64(0), core.Float64(10))

		Convey("It should score versus the anchor", func() {
			So(float64(value), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		surprise := ZScore()

		Convey("When observing", func() {
			value := surprise.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestSurprise_ObserveSample(testingTB *testing.T) {
	Convey("Given z-score", testingTB, func() {
		surprise := ZScore()
		_ = surprise.ObserveSample(0)

		Convey("When observing a step", func() {
			value := surprise.ObserveSample(10)

			Convey("It should return surprise", func() {
				So(value, ShouldBeGreaterThan, 0)
			})
		})
	})
}

func TestSurprise_ObserveSamples(testingTB *testing.T) {
	Convey("Given z-score", testingTB, func() {
		surprise := ZScore()
		samples := []float64{0, 10}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			surprise.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(out[1], ShouldBeGreaterThan, 0)
			})
		})
	})
}

func TestSurprise_Reset(testingTB *testing.T) {
	Convey("Given z-score with state", testingTB, func() {
		surprise := ZScore()
		surprise.Observe(core.Float64(3))

		Convey("When reset", func() {
			err := surprise.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(surprise.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkZScore_Observe(testingTB *testing.B) {
	surprise := ZScore()
	surprise.Observe(core.Float64(1))

	for testingTB.Loop() {
		surprise.Observe(core.Float64(1.01))
	}
}

func BenchmarkZScore_ObserveSamples(testingTB *testing.B) {
	surprise := ZScore()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		surprise.state.Reset()
		surprise.ObserveSamples(samples, out)
	}
}
