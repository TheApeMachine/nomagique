package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCalibrate_Observe(testingTB *testing.T) {
	Convey("Given a linear feature-target relation", testingTB, func() {
		calibrate, err := NewCalibrate(1, 1000)

		So(err, ShouldBeNil)

		var prediction float64

		for index := range 32 {
			feature := float64(index) / 32
			target := 2*feature + 1
			prediction = observeInputs(calibrate, feature, target)
		}

		Convey("It should converge to a small residual", func() {
			So(prediction, ShouldAlmostEqual, 2, 0.25)
		})
	})
}

func TestNewCalibrate(testingTB *testing.T) {
	Convey("Given a non-positive dimension", testingTB, func() {
		_, err := NewCalibrate(0, 1000)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkCalibrate_Observe(testingTB *testing.B) {
	calibrate, err := NewCalibrate(1, 1000)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(calibrate, 0.5, 2)
	}
}
