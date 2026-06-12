package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestSampleRatio(testingTB *testing.T) {
	Convey("Given SampleRatio constructor", testingTB, func() {
		calibrator := SampleRatio()

		Convey("It should return a usable dynamic", func() {
			So(calibrator, ShouldNotBeNil)
		})
	})
}

func TestCalibrator_Observe(testingTB *testing.T) {
	Convey("Given a fresh calibrator", testingTB, func() {
		calibrator := SampleRatio()

		Convey("When bootstrapping", func() {
			value := calibrator.Observe(core.Float64(10), core.Float64(10))

			Convey("It should return unit calibration", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})

	Convey("Given a winning outcome", testingTB, func() {
		calibrator := SampleRatio()
		calibrator.Observe(core.Float64(10), core.Float64(10))
		value := calibrator.Observe(core.Float64(10), core.Float64(15))

		Convey("It should scale by actual over predicted within the adaptive ceiling", func() {
			So(float64(value), ShouldBeGreaterThan, 1)
			So(float64(value), ShouldBeLessThanOrEqualTo, 1.5)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		calibrator := SampleRatio()

		Convey("When observing", func() {
			value := calibrator.Observe(core.Float64(0), core.Float64(10))

			Convey("It should return ErrZeroPredicted", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestCalibrator_ObserveSamples(testingTB *testing.T) {
	Convey("Given a calibrator", testingTB, func() {
		calibrator := SampleRatio()
		predicted := []float64{10, 10}
		actual := []float64{10, 15}
		out := make([]float64, len(predicted))

		Convey("When observing samples in batch", func() {
			calibrator.ObserveSamples(predicted, actual, out)

			Convey("It should fill the output buffer", func() {
				So(out[1], ShouldBeGreaterThan, 1)
			})
		})
	})
}

func BenchmarkSampleRatio_Observe(testingTB *testing.B) {
	calibrator := SampleRatio()
	calibrator.Observe(core.Float64(10), core.Float64(10))

	for testingTB.Loop() {
		calibrator.Observe(core.Float64(10), core.Float64(11))
	}
}

func BenchmarkSampleRatio_ObserveSamples(testingTB *testing.B) {
	calibrator := SampleRatio()
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	for testingTB.Loop() {
		calibrator.state.Reset()
		calibrator.ObserveSamples(predicted, actual, out)
	}
}
