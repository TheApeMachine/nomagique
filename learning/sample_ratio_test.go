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

func TestCalibratorRead(testingTB *testing.T) {
	Convey("Given empty inbound wire", testingTB, func() {
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
		_ = transport.NewFlipFlop(artifact, calibrator)

		artifact = pairWire(artifact, 10, 15)
		err := transport.NewFlipFlop(artifact, calibrator)

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

func BenchmarkSampleRatioRead(testingTB *testing.B) {
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
