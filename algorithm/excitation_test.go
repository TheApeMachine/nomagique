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

	Convey("Given a scoped feature batch before fit history is ready", testingTB, func() {
		excitation := NewExcitation()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		_, ready, err := excitation.Measure(excitationBurstInput("ALT/EUR", base, 2))

		Convey("It should stage quietly as not ready", func() {
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
