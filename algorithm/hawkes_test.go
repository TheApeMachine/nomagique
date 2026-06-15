package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/hawkes"
)

func TestHawkes_Observe(testingTB *testing.T) {
	Convey("Given parameters fit to the configured streams", testingTB, func() {
		xStream := []float64{2, 4, 6, 8}
		yStream := []float64{1, 2, 3, 4}

		seed, ok := hawkes.MethodOfMoments(xStream, yStream, nil, 1)
		So(ok, ShouldBeTrue)

		process := NewHawkes(seed, 1, 1, xStream, yStream, nil)
		confidence := observeInputs(process)

		Convey("It should report high moment-fit confidence", func() {
			So(confidence, ShouldBeGreaterThan, 0.5)
		})
	})
}

func TestHawkes_CrossAsymmetry(testingTB *testing.T) {
	Convey("Given asymmetric third-order structure between streams", testingTB, func() {
		xStream := []float64{1, 4, 9, 16}
		yStream := []float64{1, 2, 3, 4}
		process := NewHawkes(
			hawkes.BivariateParams{Beta: 1}, 1, 1, xStream, yStream, nil,
		)
		asymmetry := process.CrossAsymmetry()

		Convey("It should expose non-zero asymmetry", func() {
			So(asymmetry, ShouldNotEqual, 0)
		})
	})
}

func BenchmarkHawkes_Observe(testingTB *testing.B) {
	xStream := []float64{2, 4, 6, 8, 10, 12}
	yStream := []float64{1, 2, 3, 4, 5, 6}
	params := hawkes.BivariateParams{
		MuX:     5,
		MuY:     3,
		AlphaXX: 0.1,
		AlphaYY: 0.1,
		Beta:    1,
	}
	process := NewHawkes(params, 1, 1, xStream, yStream, nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(process)
	}
}
