package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestSampleRatio(testingTB *testing.T) {
	Convey("Given SampleRatio constructor", testingTB, func() {
		calibrator := SampleRatio[float64]()

		Convey("It should return a usable dynamic", func() {
			So(calibrator, ShouldNotBeNil)
		})
	})
}

func TestCalibrator_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		calibrator := SampleRatio[float64]()

		Convey("It should return zero output", func() {
			So(calibrator.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a fresh calibrator", testingTB, func() {
		calibrator := SampleRatio[float64]()
		got := calibrator.Observe(numberInputs(10, 10)...)

		Convey("It should return unit calibration", func() {
			So(float64(got), ShouldEqual, 1)
		})
	})

	Convey("Given a winning outcome", testingTB, func() {
		calibrator := SampleRatio[float64]()
		_ = calibrator.Observe(numberInputs(10, 10)...)
		got := calibrator.Observe(numberInputs(10, 15)...)

		Convey("It should scale by actual over predicted within the adaptive ceiling", func() {
			So(float64(got), ShouldBeGreaterThan, 1)
			So(float64(got), ShouldBeLessThanOrEqualTo, 1.5)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		calibrator := SampleRatio[float64]()
		got := calibrator.Observe(numberInputs(0, 10)...)

		Convey("It should leave output at zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})
}

func TestCalibrator_ObserveSamples(testingTB *testing.T) {
	Convey("Given a calibrator", testingTB, func() {
		calibrator := SampleRatio[float64]()
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

func TestCalibrator_Reset(testingTB *testing.T) {
	Convey("Given a calibrator with state", testingTB, func() {
		calibrator := SampleRatio[float64]()
		_ = calibrator.Observe(numberInputs(10, 10)...)

		So(calibrator.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(calibrator.state.Ready, ShouldBeFalse)
			So(float64(calibrator.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkSampleRatio_Observe(testingTB *testing.B) {
	calibrator := SampleRatio[float64]()
	_ = calibrator.Observe(numberInputs(10, 10)...)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = calibrator.Observe(numberInputs(10, 11)...)
	}
}

func BenchmarkSampleRatio_ObserveSamples(testingTB *testing.B) {
	calibrator := SampleRatio[float64]()
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		calibrator.state.Reset()
		calibrator.ObserveSamples(predicted, actual, out)
	}
}
