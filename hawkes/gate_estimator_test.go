package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFitGateEstimator_Measure(testingTB *testing.T) {
	Convey("Given enough fit history for the public gate calculation", testingTB, func() {
		radii := []float64{
			0.45, 0.48, 0.44, 0.46, 0.47, 0.45, 0.48,
			0.46, 0.44, 0.47, 0.45, 0.46, 0.48, 0.44,
		}
		asymmetries := []float64{
			0.04, 0.05, 0.06, 0.05, 0.04, 0.05, 0.06,
			0.05, 0.04, 0.05, 0.06, 0.04, 0.05, 0.05,
		}
		expected, expectedReady := FitGatesFromHistory(radii, asymmetries)
		actual, actualReady := NewFitGateEstimator().Measure(radii, asymmetries)

		Convey("It should reproduce both gates exactly", func() {
			So(actualReady, ShouldEqual, expectedReady)
			So(actual, ShouldResemble, expected)
		})
	})
}

func BenchmarkFitGateEstimator_Measure(testingTB *testing.B) {
	radii := []float64{
		0.45, 0.48, 0.44, 0.46, 0.47, 0.45, 0.48,
		0.46, 0.44, 0.47, 0.45, 0.46, 0.48, 0.44,
	}
	asymmetries := []float64{
		0.04, 0.05, 0.06, 0.05, 0.04, 0.05, 0.06,
		0.05, 0.04, 0.05, 0.06, 0.04, 0.05, 0.05,
	}
	estimator := NewFitGateEstimator()
	_, _ = estimator.Measure(radii, asymmetries)
	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = estimator.Measure(radii, asymmetries)
	}
}
