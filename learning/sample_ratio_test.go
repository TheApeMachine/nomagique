package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestSampleRatio(testingTB *testing.T) {
	Convey("Given SampleRatio constructor", testingTB, func() {
		calibrator := SampleRatio(pairConfig("sample-ratio-config"))

		Convey("It should return a usable dynamic", func() {
			So(calibrator, ShouldNotBeNil)
		})
	})
}

func TestCalibrator_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		calibrator := SampleRatio(pairConfig("sample-ratio-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, calibrator)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a fresh calibrator", testingTB, func() {
		calibrator := SampleRatio(pairConfig("sample-ratio-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 10, 10)
		err := transport.NewFlipFlop(artifact, calibrator)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return unit calibration", func() {
			So(got, ShouldEqual, 1)
		})
	})

	Convey("Given a winning outcome", testingTB, func() {
		calibrator := SampleRatio(pairConfig("sample-ratio-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact = pairWire(artifact, 10, 10)
		err := transport.NewFlipFlop(artifact, calibrator)

		So(err, ShouldBeNil)

		artifact = pairWire(artifact, 10, 15)
		err = transport.NewFlipFlop(artifact, calibrator)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should scale by actual over predicted within the adaptive ceiling", func() {
			So(got, ShouldBeGreaterThan, 1)
			So(got, ShouldBeLessThanOrEqualTo, 1.5)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		calibrator := SampleRatio(pairConfig("sample-ratio-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 0, 10)
		err := transport.NewFlipFlop(artifact, calibrator)

		Convey("It should return a parse error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestCalibrator_ObserveSamples(testingTB *testing.T) {
	Convey("Given a calibrator", testingTB, func() {
		calibrator := SampleRatio(pairConfig("sample-ratio-config"))
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
		calibrator := SampleRatio(pairConfig("sample-ratio-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 10, 10)
		err := transport.NewFlipFlop(artifact, calibrator)

		So(err, ShouldBeNil)
		So(calibrator.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](calibrator.artifact, "output", "ready"), ShouldEqual, 0)
		})

		fresh := pairWire(datura.Acquire("test", datura.APPJSON), 10, 10)
		err = transport.NewFlipFlop(fresh, calibrator)

		So(err, ShouldBeNil)

		Convey("It should observe again after reset", func() {
			So(datura.Peek[float64](calibrator.artifact, "output", "ready"), ShouldEqual, 1)
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 1)
		})
	})
}

func BenchmarkSampleRatio_Observe(testingTB *testing.B) {
	calibrator := SampleRatio(pairConfig("sample-ratio-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact = pairWire(artifact, 10, 10)
	_ = transport.NewFlipFlop(artifact, calibrator)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact = pairWire(artifact, 10, 11)
		_ = transport.NewFlipFlop(artifact, calibrator)
	}
}

func BenchmarkSampleRatio_ObserveSamples(testingTB *testing.B) {
	calibrator := SampleRatio(pairConfig("sample-ratio-config-bench"))
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = calibrator.Reset()
		calibrator.ObserveSamples(predicted, actual, out)
	}
}
