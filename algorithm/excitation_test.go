package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/hawkes"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/tests"
)

func TestExcitationMeasure(testingTB *testing.T) {
	Convey("Given a clustered buy/sell burst", testingTB, func() {
		excitation := NewExcitation()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		samples := excitationBurstSamples(base, 128)
		batch := payloadSamples(encodePayload(samples...))

		for index := range 4 {
			excitation.outcome = excitation.evaluate("ALT/EUR", batch)
			_ = index
		}

		Convey("It should publish thermal scores", func() {
			So(excitation.Outcome().Strength, ShouldBeGreaterThan, 0)
		})
	})
}

func TestExcitationClassifier(testingTB *testing.T) {
	Convey("Given excitation wired into a classifier", testingTB, func() {
		excitation := NewExcitation()
		classifier := probability.NewClassifier(
			excitation.FrenzyReading(),
			excitation.SaturationReading(),
			excitation.OrganicReading(),
			excitation.ExhaustionReading(),
		)
		pipeline := nomagique.Number(excitation, classifier)
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		inbound := daturaBurstArtifact("BTC/EUR", excitationBurstSamples(base, 128))
		frame, _ := inbound.Message().Marshal()

		_, _ = pipeline.Write(frame)
		_, _ = pipeline.Read(make([]byte, 4096))

		Convey("It should select a category", func() {
			So(classifier.CategoryIndex(), ShouldBeGreaterThan, 0)
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

		category, confidence := hawkes.ClassifyFit(fit, 0.05, false, gates)

		Convey("It should classify saturation", func() {
			So(category, ShouldEqual, hawkes.FitCategorySaturation)
			So(confidence, ShouldBeGreaterThan, 0)
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

func excitationBurstSamples(base time.Time, count int) []float64 {
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

	samples := []float64{
		horizon,
		cooldown,
		float64(len(buyTimes)),
		float64(len(sellTimes)),
	}
	samples = append(samples, buyTimes...)
	samples = append(samples, sellTimes...)

	return samples
}

func daturaBurstArtifact(scope string, samples []float64) *datura.Artifact {
	inbound := datura.Acquire("excitation-test", datura.Artifact_Type_json)
	inbound.WithScope(scope)
	_ = inbound.SetPayload(encodePayload(samples...))

	return inbound
}

func BenchmarkExcitationRead(b *testing.B) {
	excitation := NewExcitation()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	samples := excitationBurstSamples(base, 128)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(excitation, samples...)
		_, _ = excitation.Read(make([]byte, 4096))
	}
}
