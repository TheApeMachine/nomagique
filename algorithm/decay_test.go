package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestDecayMeasure(testingTB *testing.T) {
	Convey("Given deteriorating long-side book history", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayMeasureInput(
			[]float64{20, 18, 16, 14, 12, 10, 8, 6},
			stableMeasureSeries(10),
			stableMeasureSeries(8),
			stableMeasureSeries(4),
			stableMeasureSeries(0.2),
			stableMeasureSeries(0.1),
		))

		So(err, ShouldBeNil)

		Convey("It should publish an eligible exhaustion outcome", func() {
			So(output.Value, ShouldBeGreaterThan, 0)
			So(output.Urgency, ShouldBeGreaterThan, 0)
			So(int(output.Category), ShouldEqual, 1)
		})
	})

	Convey("Given pressure fade on the long side", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayMeasureInput(
			stableMeasureSeries(10),
			stableMeasureSeries(10),
			stableMeasureSeries(8),
			stableMeasureSeries(4),
			[]float64{0.9, 0.85, 0.8, 0.75, 0.7, 0.2, 0.1, -0.1},
			stableMeasureSeries(0.1),
		))

		So(err, ShouldBeNil)

		Convey("It should classify thermal exhaustion", func() {
			So(int(output.Category), ShouldEqual, 3)
		})
	})

	Convey("Given ask-side thinning stronger than bid-side", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayMeasureInput(
			[]float64{10, 10, 10, 10, 9, 9, 9, 9},
			[]float64{10, 10, 10, 10, 8, 6, 4, 2},
			stableMeasureSeries(8),
			stableMeasureSeries(4),
			stableMeasureSeries(0.2),
			stableMeasureSeries(0.1),
		))

		So(err, ShouldBeNil)

		Convey("It should let the stronger short-side score win", func() {
			So(output.Value, ShouldBeGreaterThan, 0)
			So(int(output.Category), ShouldEqual, 1)
		})
	})
}

func BenchmarkDecayMeasure(benchmark *testing.B) {
	decay := equation.NewDecay()
	input := decayMeasureInput(
		[]float64{20, 18, 16, 14, 12, 10, 8, 6},
		stableMeasureSeries(10),
		stableMeasureSeries(8),
		stableMeasureSeries(4),
		stableMeasureSeries(0.2),
		stableMeasureSeries(0.1),
	)

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = decay.Measure(input)
	}
}

func decayMeasureInput(
	bidDepths []float64,
	askDepths []float64,
	densities []float64,
	spreads []float64,
	pressures []float64,
	imbalances []float64,
) equation.DecayInput {
	return equation.DecayInput{
		LastPrice:  100,
		BidDepths:  bidDepths,
		AskDepths:  askDepths,
		Densities:  densities,
		Spreads:    spreads,
		Pressures:  pressures,
		Imbalances: imbalances,
	}
}

func stableMeasureSeries(value float64) []float64 {
	return []float64{value, value, value, value, value, value, value, value}
}
