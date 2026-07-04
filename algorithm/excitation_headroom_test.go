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
			So(frenzy, ShouldBeGreaterThan, 0)
			So(saturation, ShouldBeGreaterThan, 0)
			So(organic, ShouldBeGreaterThan, 0)
			So(exhaustion, ShouldEqual, 0)
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
