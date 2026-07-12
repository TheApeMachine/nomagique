package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/hawkes"
)

func TestExcitation_Measure(testingTB *testing.T) {
	Convey("Given a clustered buy/sell burst", testingTB, func() {
		excitation := NewExcitation()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		input := excitationBurstInput("ALT/EUR", base, 128)
		outcome := ExcitationOutcome{}
		ready := false
		var err error

		for range 4 {
			outcome, ready, err = excitation.Measure(input)

			if err == nil && ready {
				break
			}
		}

		Convey("It should publish thermal scores", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(outcome.Strength, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a scoped feature batch below the bivariate identifiability floor", testingTB, func() {
		excitation := NewExcitation()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		outcome, ready, err := excitation.Measure(excitationBurstInput("ALT/EUR", base, 2))

		Convey("It should emit the no-excitation baseline immediately, with low maturity", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(outcome.BranchingRatio, ShouldEqual, 0)
			So(outcome.SpectralRadius, ShouldEqual, 0)
			So(outcome.EventCount, ShouldEqual, 2)
			So(outcome.Maturity, ShouldBeGreaterThan, 0)
			So(outcome.Maturity, ShouldBeLessThan, 1)
		})
	})

	Convey("Given a single one-sided event", testingTB, func() {
		excitation := NewExcitation()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		_, ready, err := excitation.Measure(excitationBurstInput("ALT/EUR", base, 1))

		Convey("It should stay not-ready, since a gap between two events is the irreducible floor", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeFalse)
		})
	})

	Convey("Given a feature batch without symbol", testingTB, func() {
		excitation := NewExcitation()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		_, _, err := excitation.Measure(excitationBurstInput("", base, 32))

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestExcitation_MeasureArrivals(testingTB *testing.T) {
	Convey("Given identical incremental trades through typed and legacy excitation", testingTB, func() {
		typedSample := NewTradeExcitationSample()
		legacySample := NewTradeExcitationSample()
		typedExcitation := NewExcitation()
		legacyExcitation := NewExcitation()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		var typed ExcitationOutcome
		var legacy ExcitationOutcome
		var typedReady bool
		var legacyReady bool

		for index := range 128 {
			side := "buy"

			if index%2 == 1 {
				side = "sell"
			}

			tradeInput := tradeExcitationInput(
				"ALT/EUR",
				side,
				base.Add(time.Duration(index)*time.Millisecond),
			)
			typedInput, _, _ := typedSample.MeasureArrival(tradeInput)
			legacyInput, _, _ := legacySample.MeasureTrade(tradeInput)
			typed, typedReady, _ = typedExcitation.MeasureArrivals(typedInput)
			legacy, legacyReady, _ = legacyExcitation.Measure(legacyInput)
		}

		Convey("It should preserve fitted classification and continuous outputs", func() {
			So(typedReady, ShouldEqual, legacyReady)
			So(typed.EventCount, ShouldEqual, legacy.EventCount)
			So(typed.HighRisk, ShouldEqual, legacy.HighRisk)
			So(typed.Eligible, ShouldEqual, legacy.Eligible)
			So(typed.Frenzy, ShouldEqual, legacy.Frenzy)
			So(typed.Saturation, ShouldEqual, legacy.Saturation)
			So(typed.Organic, ShouldEqual, legacy.Organic)
			So(typed.Exhaustion, ShouldEqual, legacy.Exhaustion)
			So(typed.Strength, ShouldEqual, legacy.Strength)
			So(typed.BranchingRatio, ShouldEqual, legacy.BranchingRatio)
			So(typed.SpectralRadius, ShouldEqual, legacy.SpectralRadius)
			So(typed.BaselineMu, ShouldEqual, legacy.BaselineMu)
			So(typed.IntensityRatio, ShouldEqual, legacy.IntensityRatio)
			So(typed.Maturity, ShouldEqual, legacy.Maturity)
		})
	})
}

func TestClassifyFitSaturation(testingTB *testing.T) {
	Convey("Given a fit at critical spectral radius", testingTB, func() {
		fit := hawkes.BivariateFit{
			MuX:            1,
			MuY:            1,
			IntensityX:     2,
			IntensityY:     2,
			SpectralRadius: 0.9,
		}

		gates, gatesReady := hawkes.FitGatesFromHistory(
			[]float64{0.7, 0.75, 0.8, 0.82},
			[]float64{0.05, 0.08, 0.1, 0.12},
		)

		So(gatesReady, ShouldBeTrue)

		category, confidence, err := hawkes.ClassifyFit(fit, 0.05, false, gates)

		So(err, ShouldBeNil)

		Convey("It should classify saturation", func() {
			So(category, ShouldEqual, hawkes.FitCategorySaturation)
			So(confidence, ShouldBeGreaterThan, 0)
		})
	})
}

func TestMeasureFitFrenzy(testingTB *testing.T) {
	Convey("Given symbol-local gates from low-asymmetry history", testingTB, func() {
		symbolState := newExcitationSymbol()

		for _, sample := range []struct {
			radius, asymmetry float64
		}{
			{0.45, 0.04}, {0.48, 0.05}, {0.44, 0.06}, {0.46, 0.05},
			{0.47, 0.04}, {0.45, 0.05}, {0.48, 0.06}, {0.46, 0.05},
			{0.44, 0.04}, {0.47, 0.05}, {0.45, 0.06}, {0.46, 0.04},
			{0.48, 0.05}, {0.44, 0.05},
		} {
			symbolState.recordFitGates(sample.radius, sample.asymmetry)
		}

		gates, gatesReady := hawkes.FitGatesFromHistory(
			symbolState.spectralRadii,
			symbolState.asymmetries,
		)

		So(gatesReady, ShouldBeTrue)

		fit := hawkes.BivariateFit{
			MuX:            1,
			MuY:            1,
			IntensityX:     2,
			IntensityY:     0.5,
			SpectralRadius: gates.SaturationRadius * 0.9,
		}

		reading, ok := symbolState.measureFit(fit)

		So(ok, ShouldBeTrue)

		fitAsymmetry := fit.Asymmetry(false)
		category, _, err := hawkes.ClassifyFit(fit, fitAsymmetry, false, gates)

		So(err, ShouldBeNil)

		Convey("It should classify above the symbol frenzy gate", func() {
			So(fitAsymmetry, ShouldBeGreaterThan, gates.FrenzyAsymmetry)
			So(category, ShouldEqual, hawkes.FitCategoryFrenzy)
			So(reading.frenzy, ShouldBeGreaterThan, reading.organic)
		})
	})
}

func TestRevisionKey(testingTB *testing.T) {
	Convey("Given an arrival stream", testingTB, func() {
		start := time.Unix(1_700_000_000, 0)
		stream := hawkes.NewArrivalStream(
			[]time.Time{start, start.Add(time.Second)},
			[]time.Time{start.Add(2 * time.Second)},
		)

		Convey("It should fingerprint buy and sell bounds", func() {
			key := revisionKey(stream)
			So(key.buyCount, ShouldEqual, 2)
			So(key.sellCount, ShouldEqual, 1)
			So(key.buyFirst, ShouldEqual, start.UnixNano())
		})
	})
}

func excitationBurstInput(symbol string, base time.Time, count int) ExcitationInput {
	buyTimes := make([]float64, 0, count/2)
	sellTimes := make([]float64, 0, count/2)

	for index := range count {
		wall := base.Add(time.Duration(index) * 100 * time.Millisecond)
		seconds := float64(wall.UnixNano()) / float64(time.Second)

		if index%2 == 0 {
			sellTimes = append(sellTimes, seconds)
			continue
		}

		buyTimes = append(buyTimes, seconds)
	}

	horizon := float64(base.Add(time.Duration(count)*100*time.Millisecond).UnixNano()) / float64(time.Second)
	span := base.Add(time.Duration(count) * 100 * time.Millisecond).Sub(base)
	cooldown := DeriveFitCooldown(span).Seconds()

	return ExcitationInput{
		Symbol:             symbol,
		HorizonSeconds:     horizon,
		FitCooldownSeconds: cooldown,
		TouchImbalance:     0,
		BuySeconds:         buyTimes,
		SellSeconds:        sellTimes,
	}
}

func BenchmarkExcitation_Measure(b *testing.B) {
	excitation := NewExcitation()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	input := excitationBurstInput("ALT/EUR", base, 128)

	b.ReportAllocs()

	for b.Loop() {
		_, _, _ = excitation.Measure(input)
	}
}

func BenchmarkExcitation_MeasureArrivals(b *testing.B) {
	sample := NewTradeExcitationSample()
	excitation := NewExcitation()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	iteration := 0

	b.ReportAllocs()

	for b.Loop() {
		side := "buy"

		if iteration%2 == 1 {
			side = "sell"
		}

		input, ready, err := sample.MeasureArrival(tradeExcitationInput(
			"ALT/EUR",
			side,
			base.Add(time.Duration(iteration)*time.Millisecond),
		))

		if err != nil {
			b.Fatal(err)
		}

		if ready {
			_, _, err = excitation.MeasureArrivals(input)
		}

		if err != nil {
			b.Fatal(err)
		}

		iteration++
	}
}
