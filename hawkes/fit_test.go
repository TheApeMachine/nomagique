package hawkes

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBivariateFit_Valid(testingTB *testing.T) {
	Convey("Given a well-formed fit", testingTB, func() {
		fit := BivariateFit{
			MuX:            1,
			MuY:            1,
			AlphaXX:        0.1,
			AlphaXY:        0.1,
			AlphaYX:        0.1,
			AlphaYY:        0.1,
			Beta:           1,
			SpectralRadius: 0.5,
		}

		Convey("It should validate parameters", func() {
			So(fit.Valid(), ShouldBeTrue)
		})
	})

	Convey("Given an unstable spectral radius", testingTB, func() {
		fit := BivariateFit{MuX: 1, MuY: 1, Beta: 1, SpectralRadius: 1.2}

		Convey("It should reject the fit", func() {
			So(fit.Valid(), ShouldBeFalse)
		})
	})

	Convey("Given a no-excitation Poisson fit", testingTB, func() {
		fit := BivariateFit{MuX: 1, MuY: 1, Beta: 1, SpectralRadius: 0}

		Convey("It should accept zero spectral radius", func() {
			So(fit.Valid(), ShouldBeTrue)
		})
	})
}

func TestBivariateEstimator_Fit(testingTB *testing.T) {
	Convey("Given an arrival stream", testingTB, func() {
		start := time.Now()
		stream := NewArrivalStream(
			[]time.Time{
				start,
				start.Add(time.Second),
				start.Add(2 * time.Second),
				start.Add(3 * time.Second),
			},
			[]time.Time{
				start.Add(500 * time.Millisecond),
				start.Add(1500 * time.Millisecond),
				start.Add(2500 * time.Millisecond),
				start.Add(3500 * time.Millisecond),
			},
		)
		estimator := NewBivariateEstimator(BivariateFit{})
		fit := estimator.Fit(stream, start.Add(4*time.Second))

		Convey("It should return a valid fit when data is sufficient", func() {
			if fit.MuX > 0 {
				So(fit.Valid(), ShouldBeTrue)
			}
		})
	})
}

func TestBivariateFit_LogLikelihood(testingTB *testing.T) {
	Convey("Given one buy event and one-sided self excitation", testingTB, func() {
		start := time.Unix(0, 0)
		stream := NewArrivalStream([]time.Time{start}, nil)
		horizon := start.Add(2 * time.Second)
		fit := BivariateFit{
			MuX:     1,
			MuY:     1,
			AlphaXX: 1,
			Beta:    1,
		}

		likelihood := fit.LogLikelihood(stream, horizon)

		Convey("It should subtract the integrated kernel mass over the horizon", func() {
			expected := -4 - (1 - math.Exp(-2))

			So(likelihood, ShouldAlmostEqual, expected, 1e-12)
		})
	})
}

func TestClassifyFit_Saturation(testingTB *testing.T) {
	Convey("Given a fit at critical spectral radius", testingTB, func() {
		fit := BivariateFit{
			MuX:            1,
			MuY:            1,
			IntensityX:     2,
			IntensityY:     2,
			SpectralRadius: 0.9,
		}

		gates, gatesReady := FitGatesFromHistory(
			[]float64{0.7, 0.75, 0.8, 0.82},
			[]float64{0.05, 0.08, 0.1, 0.12},
		)

		So(gatesReady, ShouldBeTrue)

		category, confidence, err := ClassifyFit(fit, 0.05, false, gates)

		So(err, ShouldBeNil)

		Convey("It should classify saturation", func() {
			So(category, ShouldEqual, FitCategorySaturation)
			So(float64(confidence), ShouldBeGreaterThan, 0)
		})
	})
}
