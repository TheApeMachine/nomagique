package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/tests"
)

func decayPayload(
	lastPrice float64,
	bidDepths, askDepths, densities, spreads, pressures, imbalances []float64,
) []float64 {
	payload := []float64{lastPrice}

	series := [][]float64{
		bidDepths,
		askDepths,
		densities,
		spreads,
		pressures,
		imbalances,
	}

	for _, segment := range series {
		payload = append(payload, float64(len(segment)))
	}

	for _, segment := range series {
		payload = append(payload, segment...)
	}

	return payload
}

func TestDepthTrend(testingTB *testing.T) {
	Convey("Given shrinking depth samples", testingTB, func() {
		Convey("It should report positive thinning trend", func() {
			So(depthTrend([]float64{10, 10, 10, 10, 8, 6}), ShouldBeGreaterThan, 0)
		})
	})
}

func TestDecayEvaluate(testingTB *testing.T) {
	Convey("Given deteriorating long-side book history", testingTB, func() {
		decay := NewDecay()
		bidDepths := []float64{20, 18, 16, 14, 12, 10, 8, 6}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{8, 8, 8, 8, 8, 8, 8, 8}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		pressures := []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		writeErr := tests.WriteSamples(decay, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)
		So(writeErr, ShouldBeNil)
		_, _ = decay.Read(make([]byte, 4096))

		Convey("It should publish an eligible exhaustion outcome", func() {
			So(decay.outcome.Eligible, ShouldBeTrue)
			So(decay.outcome.Urgency, ShouldBeGreaterThan, 0)
			So(decay.outcome.Category, ShouldEqual, 1)
		})
	})

	Convey("Given pressure fade on the long side", testingTB, func() {
		decay := NewDecay()
		pressures := []float64{0.9, 0.85, 0.8, 0.75, 0.7, 0.2, 0.1, -0.1}
		bidDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{8, 8, 8, 8, 8, 8, 8, 8}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		writeErr := tests.WriteSamples(decay, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)
		So(writeErr, ShouldBeNil)
		_, _ = decay.Read(make([]byte, 4096))

		Convey("It should classify thermal exhaustion", func() {
			So(decay.outcome.Category, ShouldEqual, 3)
		})
	})

	Convey("Given ask-side thinning stronger than bid-side", testingTB, func() {
		decay := NewDecay()
		bidDepths := []float64{10, 10, 10, 10, 9, 9, 9, 9}
		askDepths := []float64{10, 10, 10, 10, 8, 6, 4, 2}
		densities := []float64{8, 8, 8, 8, 8, 8, 8, 8}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		pressures := []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		writeErr := tests.WriteSamples(decay, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)
		So(writeErr, ShouldBeNil)
		_, _ = decay.Read(make([]byte, 4096))

		Convey("It should let the stronger short-side score win", func() {
			So(decay.outcome.Eligible, ShouldBeTrue)
			So(decay.outcome.Category, ShouldEqual, 1)
		})
	})
}

func TestDecayClassifier(testingTB *testing.T) {
	Convey("Given a decay stage wired into a classifier", testingTB, func() {
		decay := NewDecay()
		classifier := probability.NewClassifier(
			decay.MechanicalReading(),
			decay.FragileReading(),
			decay.ThermalReading(),
			decay.ReversalReading(),
		)
		pipeline := nomagique.Number(decay, classifier)
		bidDepths := []float64{20, 18, 16, 14, 12, 10, 8, 6}

		writeErr := tests.WriteSamples(pipeline, decayPayload(
			100,
			bidDepths,
			[]float64{10, 10, 10, 10, 10, 10, 10, 10},
			[]float64{8, 8, 8, 8, 8, 8, 8, 8},
			[]float64{4, 4, 4, 4, 4, 4, 4, 4},
			[]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
			[]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		)...)
		So(writeErr, ShouldBeNil)
		_, _ = pipeline.Read(make([]byte, 4096))

		Convey("It should select a category", func() {
			So(classifier.CategoryIndex(), ShouldBeGreaterThan, 0)

			confidence, confidenceErr := classifier.Confidence(classifier.CategoryIndex())

			So(confidenceErr, ShouldBeNil)
			So(confidence, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkDecayRead(b *testing.B) {
	decay := NewDecay()
	samples := decayPayload(
		100,
		[]float64{20, 18, 16, 14, 12, 10, 8, 6},
		[]float64{10, 10, 10, 10, 10, 10, 10, 10},
		[]float64{8, 8, 8, 8, 8, 8, 8, 8},
		[]float64{4, 4, 4, 4, 4, 4, 4, 4},
		[]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
		[]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
	)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(decay, samples...)
		_, _ = decay.Read(make([]byte, 4096))
	}
}
