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
	Convey("Given a fresh exponential dynamic", testingTB, func() {
		exponential := EMA()

		Convey("When bootstrapping", func() {
			value := exponential.Observe(core.Float64(10))

			Convey("It should echo the first sample", func() {
				So(value, ShouldEqual, core.Float64(10))
			})
		})
	})
}

func TestExponential_Reset(testingTB *testing.T) {
	Convey("Given an observed exponential dynamic", testingTB, func() {
		exponential := EMA()
		_ = exponential.Observe(core.Float64(10))

		Convey("When reset", func() {
			So(exponential.Reset(), ShouldBeNil)
			value := exponential.Observe(core.Float64(20))

			Convey("It should bootstrap again", func() {
				So(value, ShouldEqual, core.Float64(20))
			})
		})
	})
}

func TestExponential_ObserveSamples(testingTB *testing.T) {
	Convey("Given a bootstrapped exponential dynamic", testingTB, func() {
		exponential := EMA()
		_ = exponential.Observe(core.Float64(10))
		samples := []float64{11, 12, 13}
		out := make([]float64, len(samples))

		exponential.ObserveSamples(samples, out)

		Convey("It should match sequential observation", func() {
			expect := EMA()
			_ = expect.Observe(core.Float64(10))

			for index, sample := range samples {
				So(out[index], ShouldEqual, float64(expect.Observe(core.Float64(sample))))
			}
		})
	})
}
