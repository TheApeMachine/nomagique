package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestExponential_ObserveSample(testingTB *testing.T) {
	Convey("Given an exponential", testingTB, func() {
		exponential := EMA()

		Convey("When observing a raw sample", func() {
			value := exponential.ObserveSample(7)

			Convey("It should return the sample", func() {
				So(value, ShouldEqual, 7)
			})
		})
	})
}

func TestNormalized_ObserveSample(testingTB *testing.T) {
	Convey("Given a normalized delta", testingTB, func() {
		delta := Delta()
		_ = delta.ObserveSample(0)

		Convey("When observing a step", func() {
			value := delta.ObserveSample(10)

			Convey("It should return one", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})
}

func TestIntegrator_ObserveSample(testingTB *testing.T) {
	Convey("Given an integrator", testingTB, func() {
		integrator := Accumulator()

		Convey("When observing a raw sample", func() {
			value := integrator.ObserveSample(7)

			Convey("It should return the integrated level", func() {
				So(value, ShouldEqual, 7)
			})
		})
	})
}

func TestCompressor_ObserveSample(testingTB *testing.T) {
	Convey("Given a compressor", testingTB, func() {
		compressor := Compression()
		_ = compressor.ObserveSample(100)

		Convey("When spread tightens", func() {
			value := compressor.ObserveSample(50)

			Convey("It should return the compression score", func() {
				So(value, ShouldEqual, 0.5)
			})
		})
	})
}

func TestFractional_ObserveSample(testingTB *testing.T) {
	Convey("Given fractional differencing", testingTB, func() {
		fractional := FracDiff()

		Convey("When observing a raw sample", func() {
			value := fractional.ObserveSample(7)

			Convey("It should return the sample on bootstrap", func() {
				So(value, ShouldEqual, 7)
			})
		})
	})
}

func TestObserveEMAThenZScore(testingTB *testing.T) {
	Convey("Given EMA and z-score", testingTB, func() {
		exponential := EMA()
		surprise := ZScore()

		Convey("When observing after bootstrap", func() {
			_ = exponential.ObserveSample(0)
			_ = surprise.ObserveSample(0)
			value := ObserveEMAThenZScore(10, exponential, surprise)

			Convey("It should return anchored surprise", func() {
				So(value, ShouldBeGreaterThan, 0)
			})
		})
	})
}

func TestObserveEMAThenDelta(testingTB *testing.T) {
	Convey("Given EMA and delta", testingTB, func() {
		exponential := EMA()
		delta := Delta()

		Convey("When observing after bootstrap", func() {
			_ = exponential.ObserveSample(0)
			_ = delta.ObserveSample(0)
			value := ObserveEMAThenDelta(10, exponential, delta)

			Convey("It should return the delta output", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})
}
