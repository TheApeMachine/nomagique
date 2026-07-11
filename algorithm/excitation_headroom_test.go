package algorithm

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/hawkes"
)

func TestOrganicHeadroomScores(t *testing.T) {
	Convey("Given ready Hawkes gates and a subcritical fit", t, func() {
		gates := hawkes.FitGates{
			SaturationRadius: 0.8,
			FrenzyAsymmetry:  0.5,
		}
		fit := hawkes.BivariateFit{
			MuX:            1.0,
			IntensityX:     1.1,
			SpectralRadius: 0.4,
		}

		frenzy, saturation, organic, exhaustion := organicHeadroomScores(
			fit,
			0.25,
			false,
			gates,
		)

		Convey("It should return the computed headroom scores", func() {
			So(frenzy, ShouldEqual, 0)
			So(saturation, ShouldEqual, 0)
			So(organic, ShouldBeGreaterThan, 0)
			So(exhaustion, ShouldEqual, 0)
		})
	})

	Convey("Given a fit beyond the excitation gates", t, func() {
		gates := hawkes.FitGates{
			SaturationRadius: 0.8,
			FrenzyAsymmetry:  0.5,
		}
		fit := hawkes.BivariateFit{
			MuX: 1, IntensityX: 2, SpectralRadius: 0.9,
		}

		frenzy, saturation, organic, _ := organicHeadroomScores(
			fit,
			0.75,
			false,
			gates,
		)

		Convey("It should score frenzy and saturation rather than organicity", func() {
			So(frenzy, ShouldBeGreaterThan, 0)
			So(saturation, ShouldBeGreaterThan, 0)
			So(organic, ShouldEqual, 0)
		})
	})
}

func TestExcitationSymbolMeasureBaseline(t *testing.T) {
	Convey("Given an arrival stream below the bivariate identifiability floor", t, func() {
		symbolState := newExcitationSymbol()
		start := time.Unix(1_700_000_000, 0)
		stream := hawkes.NewArrivalStream(
			[]time.Time{start},
			[]time.Time{start.Add(time.Second)},
		)
		horizon := start.Add(2 * time.Second)
		context, ok := hawkes.NewFitContext(stream, horizon)

		So(ok, ShouldBeTrue)
		So(context.EnoughEvents(stream), ShouldBeFalse)

		reading, readingOk := symbolState.measureBaseline(context, stream, horizon)

		Convey("It should score the no-excitation Poisson baseline, not nothing", func() {
			So(readingOk, ShouldBeTrue)
			So(reading.branchingRatio, ShouldEqual, 0)
			So(reading.spectralRadius, ShouldEqual, 0)
			So(reading.eventCount, ShouldEqual, 2)
			So(reading.maturity, ShouldBeGreaterThan, 0)
			So(reading.maturity, ShouldBeLessThan, 1)
		})

		Convey("It should never populate the cached bivariate fit", func() {
			So(symbolState.hasFit, ShouldBeFalse)
		})
	})
}

func TestExcitationReadingSeparatesBranchingRatioAndSpectralRadius(t *testing.T) {
	Convey("Given an asymmetric bivariate Hawkes fit", t, func() {
		symbolState := newExcitationSymbol()
		start := time.Unix(1_700_000_000, 0)
		stream := hawkes.NewArrivalStream(
			[]time.Time{
				start,
				start.Add(time.Second),
				start.Add(2 * time.Second),
			},
			[]time.Time{
				start.Add(3 * time.Second),
			},
		)
		fit := hawkes.BivariateFit{
			MuX:            1,
			MuY:            1,
			AlphaXX:        1,
			AlphaXY:        2,
			AlphaYX:        3,
			AlphaYY:        1,
			Beta:           10,
			IntensityX:     2,
			IntensityY:     1,
			SpectralRadius: hawkes.SpectralRadius([2][2]float64{{0.1, 0.2}, {0.3, 0.1}}),
		}

		reading, ok := symbolState.enrichReading(
			excitationReading{strength: 1, highRisk: true},
			true,
			fit,
			stream,
			start.Add(5*time.Second),
		)
		outcome := excitationOutcomeFromReading(reading)

		Convey("It should publish distinct Hawkes branching and stability metrics", func() {
			So(ok, ShouldBeTrue)
			So(reading.branchingRatio, ShouldAlmostEqual, 0.375)
			So(reading.spectralRadius, ShouldAlmostEqual, fit.SpectralRadius)
			So(math.Abs(reading.branchingRatio-reading.spectralRadius), ShouldBeGreaterThan, 0.01)
			So(outcome.BranchingRatio, ShouldAlmostEqual, reading.branchingRatio)
			So(outcome.SpectralRadius, ShouldAlmostEqual, reading.spectralRadius)
		})
	})
}
