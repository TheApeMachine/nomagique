package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func calibrateConfig(dimension int, initialVariance float64) *datura.Artifact {
	return datura.Acquire("calibrate-config", datura.APPJSON).
		WithAttribute("dimension", float64(dimension)).
		WithAttribute("initialVariance", initialVariance)
}

func TestCalibrateRead(testingTB *testing.T) {
	Convey("Given a linear feature-target relation", testingTB, func() {
		calibrate := NewCalibrate(calibrateConfig(1, 1000))

		var prediction float64
		lastTarget := 0.0

		for index := range 32 {
			feature := float64(index) / 32
			target := 2*feature + 1
			lastTarget = target
			prediction = observeInputs(calibrate, feature, target)
		}

		Convey("It should converge to a small residual", func() {
			So(prediction, ShouldAlmostEqual, lastTarget, 0.25)
		})
	})

	Convey("Given a non-positive dimension", testingTB, func() {
		calibrate := NewCalibrate(calibrateConfig(0, 1000))
		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{1, 2}, "batch")
		err := transport.NewFlipFlop(artifact, calibrate)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkCalibrateRead(testingTB *testing.B) {
	calibrate := NewCalibrate(calibrateConfig(1, 1000))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(calibrate, 0.5, 2)
	}
}
