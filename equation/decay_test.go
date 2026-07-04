package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestDecay_Measure(testingTB *testing.T) {
	Convey("Given deteriorating long-side book history", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayInput(
			[]float64{20, 18, 16, 14, 12, 10, 8, 6},
			[]float64{10, 10, 10, 10, 10, 10, 10, 10},
			[]float64{8, 8, 8, 8, 8, 8, 8, 8},
			[]float64{4, 4, 4, 4, 4, 4, 4, 4},
			[]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
			[]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		))

		So(err, ShouldBeNil)

		Convey("It should classify mechanical exhaustion", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.Value, ShouldBeGreaterThan, 0)
			So(output.Urgency, ShouldEqual, output.Value)
		})
	})

	Convey("Given pressure fade on the long side", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayInput(
			stableSeries(10),
			stableSeries(10),
			stableSeries(8),
			stableSeries(4),
			[]float64{0.9, 0.85, 0.8, 0.75, 0.7, 0.2, 0.1, -0.1},
			stableSeries(0.1),
		))

		So(err, ShouldBeNil)

		Convey("It should classify thermal exhaustion", func() {
			So(int(output.Category), ShouldEqual, 3)
		})
	})

	Convey("Given widening spread without depth collapse", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayInput(
			stableSeries(10),
			stableSeries(10),
			stableSeries(20),
			[]float64{1, 1, 1, 1, 1, 1, 1, 3},
			stableSeries(0.2),
			stableSeries(0.1),
		))

		So(err, ShouldBeNil)

		Convey("It should classify fragile expansion", func() {
			So(int(output.Category), ShouldEqual, 2)
			So(output.Fragile, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given stronger mechanical decay with moderate spread widening", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayInput(
			[]float64{10, 10, 10, 10, 0, 0, 0, 0},
			stableSeries(10),
			stableSeries(20),
			[]float64{10, 10, 10, 10, 10, 10, 10, 16},
			stableSeries(0.2),
			stableSeries(0.1),
		))

		So(err, ShouldBeNil)

		Convey("It should compare category scores on the same scale", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.Mechanical, ShouldBeGreaterThan, output.Fragile)
		})
	})

	Convey("Given support-side imbalance flipping against the position", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayInput(
			stableSeries(10),
			stableSeries(10),
			stableSeries(20),
			stableSeries(1),
			stableSeries(0.2),
			[]float64{0.4, 0.35, 0.3, 0.32, 0.28, 0.3, 0.25, -0.5},
		))

		So(err, ShouldBeNil)

		Convey("It should classify active reversal", func() {
			So(int(output.Category), ShouldEqual, 4)
			So(output.Reversal, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given stable book history without decay signals", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(decayInput(
			stableSeries(10),
			stableSeries(10),
			stableSeries(20),
			stableSeries(4),
			stableSeries(0),
			stableSeries(0.1),
		))

		So(err, ShouldBeNil)

		Convey("It should emit zero urgency without rejecting the stage", func() {
			So(int(output.Category), ShouldEqual, 0)
			So(output.Urgency, ShouldEqual, 0)
			So(output.Mechanical, ShouldEqual, 0)
			So(output.Fragile, ShouldEqual, 0)
			So(output.Thermal, ShouldEqual, 0)
			So(output.Reversal, ShouldEqual, 0)
		})
	})
}

func BenchmarkDecayMeasure(benchmark *testing.B) {
	decay := equation.NewDecay()
	input := decayInput(
		[]float64{20, 18, 16, 14, 12, 10, 8, 6},
		stableSeries(10),
		stableSeries(8),
		stableSeries(4),
		stableSeries(0.2),
		stableSeries(0.1),
	)

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = decay.Measure(input)
	}
}

func decayInput(
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

func stableSeries(value float64) []float64 {
	return []float64{value, value, value, value, value, value, value, value}
}
