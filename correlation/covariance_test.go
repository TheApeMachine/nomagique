package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCovariance_Observe(testingTB *testing.T) {
	Convey("Given positively coupled streams", testingTB, func() {
		covariance := NewCovariance(nil)
		got := observeSplit(covariance,
			[]float64{1, 2, 3, 4},
			[]float64{2, 4, 6, 8},
		)

		Convey("It should return positive covariance", func() {
			So(float64(got), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given empty Observe inputs", testingTB, func() {
		covariance := NewCovariance(nil)

		Convey("It should return zero output", func() {
			So(observeInputs(covariance), ShouldEqual, 0)
		})
	})
}

func TestCovariance_Reset(testingTB *testing.T) {
	Convey("Given an observed covariance stage", testingTB, func() {
		covariance := NewCovariance(nil)
		_ = observeSplit(covariance,
			[]float64{1, 2, 3, 4},
			[]float64{2, 4, 6, 8},
		)

		So(covariance.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(observeInputs(covariance)), ShouldEqual, 0)
		})
	})
}

func BenchmarkCovariance_Observe(testingTB *testing.B) {
	covariance := NewCovariance(nil)
	for testingTB.Loop() {
		_ = observeSplit(covariance,
			[]float64{1, 2, 3, 4, 5, 6, 7, 8},
			[]float64{2, 4, 6, 8, 10, 12, 14, 16},
		)
	}
}
