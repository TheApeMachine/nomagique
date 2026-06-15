package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCalibrate_Observe(testingTB *testing.T) {
	Convey("Given a linear feature-target relation", testingTB, func() {
		feature := make([]float64, 32)
		target := make([]float64, 32)

		for index := range feature {
			feature[index] = float64(index) / 32
			target[index] = 2*feature[index] + 1
		}

		calibrate, err := NewCalibrate[float64](
			[][]float64{feature},
			target,
			1000,
			1,
		)

		So(err, ShouldBeNil)

		residual := calibrate.Observe()

		Convey("It should converge to a small residual", func() {
			So(float64(residual), ShouldBeLessThan, 0.25)
			So(len(calibrate.Coefficients()), ShouldEqual, 2)
			So(float64(calibrate.ConditionNumber()), ShouldBeGreaterThan, 0)
		})
	})
}

func TestNewCalibrate(testingTB *testing.T) {
	Convey("Given no feature streams", testingTB, func() {
		_, err := NewCalibrate[float64](nil, []float64{1}, 1000, 1)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkCalibrate_Observe(testingTB *testing.B) {
	feature := make([]float64, 32)
	target := make([]float64, 32)

	for index := range feature {
		feature[index] = float64(index) / 32
		target[index] = 2*feature[index] + 1
	}

	calibrate, err := NewCalibrate[float64](
		[][]float64{feature},
		target,
		1000,
		1,
	)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = calibrate.Observe()
	}
}
