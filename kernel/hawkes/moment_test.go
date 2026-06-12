package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBivariateParams_MeanIntensity(testingTB *testing.T) {
	Convey("Given a stable parameter set", testingTB, func() {
		params := BivariateParams{
			MuBuy:   1,
			MuSell:  2,
			AlphaBB: 0.2,
			AlphaSS: 0.3,
			Beta:    1,
		}

		buy, sell, ok := params.MeanIntensity()

		Convey("It should recover positive intensities", func() {
			So(ok, ShouldBeTrue)
			So(buy, ShouldBeGreaterThan, 0)
			So(sell, ShouldBeGreaterThan, 0)
		})
	})
}

func TestMethodOfMoments(testingTB *testing.T) {
	Convey("Given proportional buy and sell count streams", testingTB, func() {
		buy := []float64{2, 4, 6, 8}
		sell := []float64{1, 2, 3, 4}
		params, ok := MethodOfMoments(buy, sell, nil, 1)

		Convey("It should return a stable seed", func() {
			So(ok, ShouldBeTrue)
			So(params.Stable(), ShouldBeTrue)
			So(params.MuBuy, ShouldBeGreaterThan, 0)
			So(params.MuSell, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkMethodOfMoments(testingTB *testing.B) {
	buy := []float64{2, 4, 6, 8, 10, 12}
	sell := []float64{1, 2, 3, 4, 5, 6}

	for testingTB.Loop() {
		_, _ = MethodOfMoments(buy, sell, nil, 1)
	}
}
