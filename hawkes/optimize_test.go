package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLogParamsFromFit(t *testing.T) {
	Convey("Given a fitted model", t, func() {
		fit := BivariateFit{
			MuX:     2,
			MuY:     3,
			AlphaXX: 0.4,
			AlphaXY: 0.1,
			AlphaYX: 0.1,
			AlphaYY: 0.5,
			Beta:    2,
		}
		logParams, ok := logParamsFromFit(fit)

		So(ok, ShouldBeTrue)

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

func TestMultiStartSeeds(t *testing.T) {
	Convey("Given an estimator and fit context", t, func() {
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
