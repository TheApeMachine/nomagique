package hawkes

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gonum.org/v1/gonum/stat"
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

func TestMethodOfMomentsStationarySeedIncludesCrossExcitation(testingTB *testing.T) {
	Convey("Given correlated x and y count streams", testingTB, func() {
		x := []float64{1.0, 1.2, 0.9, 1.1, 1.0, 1.15}
		y := []float64{0.8, 1.1, 0.9, 1.2, 1.0, 1.05}
		beta := 10.0
		params, ok := MethodOfMoments(x, y, nil, beta)

		Convey("It should solve mu from the full bivariate branching matrix", func() {
			So(ok, ShouldBeTrue)

			meanX := stat.Mean(x, nil)
			meanY := stat.Mean(y, nil)
			branchXX := params.AlphaXX / beta
			branchXY := params.AlphaXY / beta
			branchYX := params.AlphaYX / beta
			branchYY := params.AlphaYY / beta

			So(params.MuX, ShouldAlmostEqual, meanX-branchXX*meanX-branchXY*meanY, 1e-12)
			So(params.MuY, ShouldAlmostEqual, meanY-branchYX*meanX-branchYY*meanY, 1e-12)
			So(math.Abs(branchXY*meanY), ShouldBeGreaterThan, 0)
			So(math.Abs(branchYX*meanX), ShouldBeGreaterThan, 0)
		})
	})
}

func TestMomentMeasureOutputsBranchingEstimate(testingTB *testing.T) {
	Convey("Given a moment diagnostic with aligned samples", testingTB, func() {
		moment, err := NewMoment(MomentConfig{
			Params:  BivariateParams{MuX: 1, MuY: 1, AlphaXX: 0.1, Beta: 1},
			MomentR: 2,
		})
		So(err, ShouldBeNil)

		output, err := moment.Measure(MomentInput{
			X: []float64{2, 4, 6, 8},
			Y: []float64{1, 2, 3, 4},
		})
		So(err, ShouldBeNil)

		Convey("It should name the diagnostic as an estimate", func() {
			So(output.Value, ShouldBeGreaterThan, 0)
			So(output.Estimate, ShouldBeGreaterThan, 0)
			So(output.Confidence, ShouldEqual, output.Value)
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

func BenchmarkMomentMeasure(testingTB *testing.B) {
	moment, err := NewMoment(MomentConfig{
		Params:  BivariateParams{MuX: 1, MuY: 1, Beta: 1},
		MomentR: 1,
		MomentS: 1,
	})
	if err != nil {
		testingTB.Fatalf("new moment: %v", err)
	}

	input := MomentInput{
		X: []float64{2, 4, 6, 8},
		Y: []float64{1, 2, 3, 4},
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = moment.Measure(input)
	}
}
