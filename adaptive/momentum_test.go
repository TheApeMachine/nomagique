package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestMomentum(testingTB *testing.T) {
	Convey("Given Momentum constructor", testingTB, func() {
		impulse := Momentum()

		Convey("It should return a usable dynamic", func() {
			So(impulse, ShouldNotBeNil)
		})
	})
}

func TestImpulse_Observe(testingTB *testing.T) {
	Convey("Given a fresh momentum dynamic", testingTB, func() {
		impulse := Momentum()

		Convey("When bootstrapping", func() {
			value := impulse.Observe(core.Float64(0))

			Convey("It should return zero without error", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given momentum history", testingTB, func() {
		impulse := Momentum()
		impulse.Observe(core.Float64(0))
		value := impulse.Observe(core.Float64(10))

		Convey("It should return positive momentum", func() {
			So(value, ShouldEqual, 1)
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		impulse := Momentum()

		Convey("When observing", func() {
			value := impulse.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestImpulse_ObserveSample(testingTB *testing.T) {
	Convey("Given momentum", testingTB, func() {
		impulse := Momentum()
		_ = impulse.ObserveSample(0)

		Convey("When price rises", func() {
			value := impulse.ObserveSample(10)

			Convey("It should return unit momentum", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})
}

func TestImpulse_ObserveSamples(testingTB *testing.T) {
	Convey("Given momentum", testingTB, func() {
		impulse := Momentum()
		samples := []float64{0, 10}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			impulse.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(out[1], ShouldEqual, 1)
			})
		})
	})
}

func TestImpulse_Reset(testingTB *testing.T) {
	Convey("Given momentum with state", testingTB, func() {
		impulse := Momentum()
		impulse.Observe(core.Float64(3))

		Convey("When reset", func() {
			err := impulse.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(impulse.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkMomentum_Observe(testingTB *testing.B) {
	impulse := Momentum()
	impulse.Observe(core.Float64(1))

	for testingTB.Loop() {
		impulse.Observe(core.Float64(1.01))
	}
}

func BenchmarkMomentum_ObserveSamples(testingTB *testing.B) {
	impulse := Momentum()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		impulse.state.Reset()
		impulse.ObserveSamples(samples, out)
	}
}
