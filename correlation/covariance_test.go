package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestCovariance_Observe(testingTB *testing.T) {
	Convey("Given positively coupled streams", testingTB, func() {
		covariance := NewCovariance[float64](nil)
		got := covariance.Observe(splitInputs(
			[]float64{1, 2, 3, 4},
			[]float64{2, 4, 6, 8},
		)...)

		Convey("It should return positive covariance", func() {
			So(float64(got), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given empty Observe inputs", testingTB, func() {
		covariance := NewCovariance[float64](nil)

		Convey("It should return zero output", func() {
			So(covariance.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})
}

func TestCovariance_Reset(testingTB *testing.T) {
	Convey("Given an observed covariance stage", testingTB, func() {
		covariance := NewCovariance[float64](nil)
		_ = covariance.Observe(splitInputs(
			[]float64{1, 2, 3, 4},
			[]float64{2, 4, 6, 8},
		)...)

		So(covariance.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(covariance.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkCovariance_Observe(testingTB *testing.B) {
	covariance := NewCovariance[float64](nil)
	inputs := splitInputs(
		[]float64{1, 2, 3, 4, 5, 6, 7, 8},
		[]float64{2, 4, 6, 8, 10, 12, 14, 16},
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = covariance.Observe(inputs...)
	}
}
