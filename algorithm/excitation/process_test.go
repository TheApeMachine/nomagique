package excitation

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/hawkes"
)

func TestProcess_Measure(t *testing.T) {
	Convey("Given one event without an identifiable interval", t, func() {
		process := NewProcess()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		outcome, ready, err := process.Measure(Input{
			Symbol:  "ALT/EUR",
			Horizon: base,
			Stream:  hawkes.NewArrivalStream([]time.Time{base}, nil),
		})

		Convey("It should publish only the count observation", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(outcome.Readiness.Observation, ShouldBeTrue)
			So(outcome.Readiness.Intensity, ShouldBeFalse)
			So(outcome.EventCount, ShouldEqual, 1)
		})
	})

	Convey("Given a stream without a symbol", t, func() {
		process := NewProcess()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		_, _, err := process.Measure(Input{
			Horizon: base,
			Stream:  hawkes.NewArrivalStream([]time.Time{base}, nil),
		})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given enough typed arrivals to identify the exponential model", t, func() {
		process := NewProcess()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		stream, horizon := burstStream(base, 32)
		outcome, ready, err := process.Measure(Input{
			Symbol:  "ALT/EUR",
			Horizon: horizon,
			Stream:  stream,
		})

		Convey("It should publish fitted state without promoting it to a forecast", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(outcome.Readiness.Intensity, ShouldBeTrue)
			So(outcome.Readiness.HawkesFit, ShouldBeTrue)
			So(outcome.Readiness.ModelUpdated, ShouldBeTrue)
			So(outcome.Readiness.Forecast, ShouldBeFalse)
			So(outcome.Fit.Valid(), ShouldBeTrue)
			So(outcome.Maturity, ShouldEqual, 1.0)
		})

		Convey("When the unchanged stream is measured again", func() {
			projected, projectedReady, projectedErr := process.Measure(
				Input{
					Symbol:  "ALT/EUR",
					Horizon: horizon,
					Stream:  stream,
				},
			)

			Convey("It should reuse the fit only for current conditional intensity", func() {
				So(projectedErr, ShouldBeNil)
				So(projectedReady, ShouldBeTrue)
				So(projected.Readiness.HawkesFit, ShouldBeTrue)
				So(projected.Readiness.ModelUpdated, ShouldBeFalse)
				So(projected.FitAt, ShouldResemble, outcome.FitAt)
			})
		})

		Convey("It should compare full, Poisson, and re-estimated self-only likelihoods", func() {
			context, contextReady := hawkes.NewObservationContext(stream, horizon)
			So(contextReady, ShouldBeTrue)
			poisson := context.PoissonFit().WithIntensitiesAt(stream, horizon)
			selfOnly := hawkes.NewBivariateEstimator(hawkes.BivariateFit{}).
				FitSelfOnly(stream, horizon)
			fullLikelihood := outcome.Fit.LogLikelihood(stream, horizon)

			So(outcome.HawkesPoissonLogLikelihoodDelta, ShouldAlmostEqual,
				fullLikelihood-poisson.LogLikelihood(stream, horizon), 1e-9)
			So(outcome.CrossSelfLogLikelihoodDelta, ShouldAlmostEqual,
				fullLikelihood-selfOnly.LogLikelihood(stream, horizon), 1e-9)
		})
	})

	Convey("Given a dense one-sided stream", t, func() {
		process := NewProcess()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		buyTimes := make([]time.Time, 32)

		for index := range buyTimes {
			buyTimes[index] = base.Add(time.Duration(index) * time.Millisecond)
		}

		horizon := base.Add(32 * time.Millisecond)
		outcome, ready, err := process.Measure(Input{
			Symbol:  "ALT/EUR",
			Horizon: horizon,
			Stream:  hawkes.NewArrivalStream(buyTimes, nil),
		})

		Convey("It should retain empirical rates without inventing the empty side", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(outcome.Readiness.Intensity, ShouldBeTrue)
			So(outcome.Readiness.HawkesFit, ShouldBeFalse)
			So(outcome.BuyArrivalRate, ShouldBeGreaterThan, 0)
			So(outcome.SellArrivalRate, ShouldEqual, 0)
			So(outcome.Maturity, ShouldEqual, 0)
			So(outcome.Fit.MuX, ShouldEqual, 0)
			So(outcome.Fit.MuY, ShouldEqual, 0)
			So(outcome.Readiness.Reason, ShouldContainSubstring, "per side")
		})
	})

	Convey("Given a fitted sixteen-event epoch", t, func() {
		process := NewProcess()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		stream, horizon := burstStream(base, 16)
		initial, ready, err := process.Measure(Input{
			Symbol:  "ALT/EUR",
			Horizon: horizon,
			Stream:  stream,
		})

		So(err, ShouldBeNil)
		So(ready, ShouldBeTrue)
		So(initial.Readiness.ModelUpdated, ShouldBeTrue)

		Convey("It should refit at the square-root sampling-error scale", func() {
			var outcome Outcome

			for added := 1; added <= 4; added++ {
				stream, horizon = burstStream(base, 16+added)
				outcome, ready, err = process.Measure(Input{
					Symbol:  "ALT/EUR",
					Horizon: horizon,
					Stream:  stream,
				})

				So(err, ShouldBeNil)
				So(ready, ShouldBeTrue)

				if added < 4 {
					So(outcome.Readiness.ModelUpdated, ShouldBeFalse)
				}
			}

			So(outcome.Readiness.ModelUpdated, ShouldBeTrue)
		})
	})
}

func burstStream(base time.Time, count int) (hawkes.ArrivalStream, time.Time) {
	buyTimes := make([]time.Time, 0, count/2)
	sellTimes := make([]time.Time, 0, count/2)

	for index := range count {
		eventTime := base.Add(time.Duration(index) * 100 * time.Millisecond)

		if index%2 == 0 {
			sellTimes = append(sellTimes, eventTime)

			continue
		}

		buyTimes = append(buyTimes, eventTime)
	}

	return hawkes.NewArrivalStream(buyTimes, sellTimes),
		base.Add(time.Duration(count) * 100 * time.Millisecond)
}

func BenchmarkProcess_Measure(t *testing.B) {
	process := NewProcess()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	stream, horizon := burstStream(base, 128)
	input := Input{
		Symbol:  "ALT/EUR",
		Horizon: horizon,
		Stream:  stream,
	}

	t.ReportAllocs()

	for t.Loop() {
		_, _, _ = process.Measure(input)
	}
}
