package hawkes

import "testing"

import . "github.com/smartystreets/goconvey/convey"

func TestBivariateParams_BranchingMatrix(t *testing.T) {
	Convey("Given exponential-kernel amplitudes and decay", t, func() {
		params := BivariateParams{
			AlphaXX: 0.4,
			AlphaXY: 0.2,
			AlphaYX: 0.6,
			AlphaYY: 0.8,
			Beta:    2,
		}

		Convey("It should return direct offspring by child row and parent column", func() {
			So(params.BranchingMatrix(), ShouldResemble, [2][2]float64{
				{0.2, 0.1},
				{0.3, 0.4},
			})
		})
	})

	Convey("Given a Poisson process without excitation or a decay kernel", t, func() {
		Convey("It should have a zero branching matrix by definition", func() {
			So(BivariateParams{}.BranchingMatrix(), ShouldResemble,
				[2][2]float64{})
		})
	})
}

func TestSpectralRadius(testingTB *testing.T) {
	matrix := [2][2]float64{
		{0.4, 0.1},
		{0.2, 0.3},
	}
	radius := SpectralRadius(matrix)

	if radius <= 0 || radius >= 1 {
		testingTB.Fatalf("expected subcritical radius, got %v", radius)
	}
}

func BenchmarkBivariateParams_BranchingMatrix(testingTB *testing.B) {
	params := BivariateParams{
		AlphaXX: 0.4,
		AlphaXY: 0.2,
		AlphaYX: 0.6,
		AlphaYY: 0.8,
		Beta:    2,
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = params.BranchingMatrix()
	}
}
