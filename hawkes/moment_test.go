package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBivariateParams_MeanIntensity(testingTB *testing.T) {
	Convey("Given a stable parameter set", testingTB, func() {
		params := BivariateParams{
			MuX:     1,
			MuY:     2,
			AlphaXX: 0.2,
			AlphaYY: 0.3,
			Beta:    1,
		}

		lambdaX, lambdaY, ok := params.MeanIntensity()

		Convey("It should recover positive intensities", func() {
			So(ok, ShouldBeTrue)
			So(lambdaX, ShouldBeGreaterThan, 0)
			So(lambdaY, ShouldBeGreaterThan, 0)
		})
	})
}

func TestMethodOfMoments(testingTB *testing.T) {
	Convey("Given proportional x and y count streams", testingTB, func() {
		x := []float64{2, 4, 6, 8}
		y := []float64{1, 2, 3, 4}
		params, ok := MethodOfMoments(x, y, nil, 1)

		Convey("It should return a stable seed", func() {
			So(ok, ShouldBeTrue)
			So(params.Stable(), ShouldBeTrue)
			So(params.MuX, ShouldBeGreaterThan, 0)
			So(params.MuY, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkMethodOfMoments(testingTB *testing.B) {
	x := []float64{2, 4, 6, 8, 10, 12}
	y := []float64{1, 2, 3, 4, 5, 6}

	for testingTB.Loop() {
		_, _ = MethodOfMoments(x, y, nil, 1)
	}
}
