package hawkes

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBivariateFit_LogLikelihoodGradient(testingTB *testing.T) {
	Convey("Given a valid fit and arrival stream", testingTB, func() {
		start := time.Now()
		stream := NewArrivalStream(
			[]time.Time{
				start,
				start.Add(time.Second),
				start.Add(2 * time.Second),
			},
			[]time.Time{
				start.Add(500 * time.Millisecond),
				start.Add(1500 * time.Millisecond),
			},
		)
		fit := BivariateFit{
			MuX:            1,
			MuY:            1,
			AlphaXX:        0.1,
			AlphaXY:        0.05,
			AlphaYX:        0.05,
			AlphaYY:        0.1,
			Beta:           1,
			SpectralRadius: 0.5,
		}
		horizon := start.Add(3 * time.Second)

		logLikelihood, gradient, ok := fit.LogLikelihoodGradient(stream, horizon)

		Convey("It should return finite likelihood and gradients", func() {
			So(ok, ShouldBeTrue)
			So(math.IsInf(logLikelihood, 0), ShouldBeFalse)
			So(math.IsNaN(logLikelihood), ShouldBeFalse)
			So(math.IsNaN(gradient[0]), ShouldBeFalse)
		})
	})
}

func TestKernelSupportBetaDerivative(testingTB *testing.T) {
	Convey("Given one buy event before horizon", testingTB, func() {
		start := time.Unix(0, 0)
		stream := NewArrivalStream([]time.Time{start}, nil)
		horizon := start.Add(2 * time.Second)

		derivative := kernelSupportBetaDerivative(stream.buy, horizon, 1)

		Convey("It should be positive", func() {
			So(derivative, ShouldBeGreaterThan, 0)
		})
	})
}
