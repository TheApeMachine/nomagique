package hawkes

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSoftplus(testingTB *testing.T) {
	Convey("Given softplus and its inverse", testingTB, func() {
		value := softplus(0.5)
		roundTrip := inverseSoftplus(value)

		Convey("It should round-trip in log space", func() {
			So(roundTrip, ShouldAlmostEqual, 0.5, 1e-9)
		})
	})
}

func TestLogParamBounds_encodeDecode(testingTB *testing.T) {
	Convey("Given fit context bounds", testingTB, func() {
		context := FitContext{
			SpanSec:        10,
			TotalEvents:    20,
			EventsX:        10,
			EventsY:        10,
			MedianGapSec:   0.5,
			BetaCandidates: []float64{0.5, 1, 2},
			BranchFloor:    0.01,
			BranchCeiling:  0.9,
			LocalScales:    []float64{0.5},
		}
		bounds := context.logParamBounds()
		start := bounds.encode([bivariateParamCount]float64{
			0, 0, 0, -2, -3, -3, -2,
		})
		decoded := bounds.decode(start)

		Convey("It should stay inside configured bounds", func() {
			for index := range decoded {
				So(decoded[index], ShouldBeGreaterThanOrEqualTo, bounds.lower[index])
				So(decoded[index], ShouldBeLessThanOrEqualTo, bounds.upper[index])
			}
		})
	})
}

func TestFitFromLogParams(testingTB *testing.T) {
	Convey("Given in-range log parameters", testingTB, func() {
		context := FitContext{
			BranchFloor:   0.01,
			BranchCeiling: 0.9,
		}
		fit := fitFromLogParams([bivariateParamCount]float64{
			math.Log(1),
			math.Log(1),
			math.Log(1),
			math.Log(0.2),
			math.Log(0.05),
			math.Log(0.05),
			math.Log(0.2),
		}, context)

		Convey("It should produce a valid fit", func() {
			So(fit.MuX, ShouldBeGreaterThan, 0)
			So(fit.Beta, ShouldBeGreaterThan, 0)
			So(fit.SpectralRadius, ShouldBeGreaterThan, 0)
			So(fit.SpectralRadius, ShouldBeLessThan, criticalBranch)
		})
	})
}

func TestLogParamsFromFit(testingTB *testing.T) {
	Convey("Given a fitted model", testingTB, func() {
		fit := BivariateFit{
			MuX:     2,
			MuY:     3,
			AlphaXX: 0.4,
			AlphaXY: 0.1,
			AlphaYX: 0.1,
			AlphaYY: 0.5,
			Beta:    2,
		}

		logParams := logParamsFromFit(fit)
		rebuilt := fitFromLogParams(logParams, FitContext{
			BranchFloor:   0.01,
			BranchCeiling: 0.9,
		})

		Convey("It should preserve branch intensities", func() {
			So(rebuilt.AlphaXX, ShouldAlmostEqual, fit.AlphaXX, 1e-9)
			So(rebuilt.AlphaYY, ShouldAlmostEqual, fit.AlphaYY, 1e-9)
		})
	})
}

func TestMultiStartSeeds(testingTB *testing.T) {
	Convey("Given an estimator and fit context", testingTB, func() {
		estimator := NewBivariateEstimator(BivariateFit{})
		context := FitContext{
			SpanSec:        10,
			TotalEvents:    20,
			EventsX:        10,
			EventsY:        10,
			MedianGapSec:   0.5,
			BetaCandidates: []float64{0.5, 1, 2},
			BranchFloor:    0.01,
			BranchCeiling:  0.9,
			LocalScales:    []float64{0.5, 2},
		}

		seeds := estimator.multiStartSeeds(context)

		Convey("It should include base and scaled seeds", func() {
			So(len(seeds), ShouldBeGreaterThan, 1)
		})
	})
}
