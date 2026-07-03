package hawkes

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBivariateFit_LogLikelihoodGradient(testingTB *testing.T) {
	Convey("Given a valid fit and arrival stream", testingTB, func() {
		start := time.Unix(0, 0)
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

		Convey("It should match finite-difference likelihood derivatives", func() {
			So(ok, ShouldBeTrue)

			epsilon := 1e-5

			for index := range gradient {
				left := fit
				right := fit

				switch index {
				case 0:
					left.MuX -= epsilon
					right.MuX += epsilon
				case 1:
					left.MuY -= epsilon
					right.MuY += epsilon
				case 2:
					left.AlphaXX -= epsilon
					right.AlphaXX += epsilon
				case 3:
					left.AlphaXY -= epsilon
					right.AlphaXY += epsilon
				case 4:
					left.AlphaYX -= epsilon
					right.AlphaYX += epsilon
				case 5:
					left.AlphaYY -= epsilon
					right.AlphaYY += epsilon
				case 6:
					left.Beta -= epsilon
					right.Beta += epsilon
				}

				numerical := (right.LogLikelihood(stream, horizon) - left.LogLikelihood(stream, horizon)) / (2 * epsilon)

				So(gradient[index], ShouldAlmostEqual, numerical, 1e-5)
			}
		})
	})
}

func TestKernelIntegralSupportBetaDerivative(testingTB *testing.T) {
	Convey("Given one buy event before horizon", testingTB, func() {
		start := time.Unix(0, 0)
		stream := NewArrivalStream([]time.Time{start}, nil)
		horizon := start.Add(2 * time.Second)

		derivative := kernelIntegralSupportBetaDerivative(stream.buy, horizon, 1)

		Convey("It should match d/dbeta of one minus exponential survival", func() {
			So(derivative, ShouldAlmostEqual, 2*math.Exp(-2), 1e-12)
		})
	})
}
