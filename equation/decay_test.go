package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestDecay_Measure(testingTB *testing.T) {
	Convey("Given a bid depth ratio well below its own baseline", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:     100,
			BidDepthRatio: 0.3,
			AskDepthRatio: 1,
			DensityRatio:  0.9,
		})

		So(err, ShouldBeNil)

		Convey("It should classify mechanical exhaustion", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.Value, ShouldBeGreaterThan, 0)
			So(output.Urgency, ShouldEqual, output.Value)
		})
	})

	Convey("Given trade pressure well below its running peak", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:      100,
			BidDepthRatio:  1,
			AskDepthRatio:  1,
			DensityRatio:   1,
			Pressure:       -0.1,
			PressurePeak:   0.9,
			PressureTrough: -0.1,
		})

		So(err, ShouldBeNil)

		Convey("It should classify thermal exhaustion", func() {
			So(int(output.Category), ShouldEqual, 3)
			So(output.Thermal, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a spread deviation without depth collapse", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:       100,
			BidDepthRatio:   1,
			AskDepthRatio:   1,
			DensityRatio:    1,
			SpreadDeviation: 2.5,
		})

		So(err, ShouldBeNil)

		Convey("It should classify fragile expansion", func() {
			So(int(output.Category), ShouldEqual, 2)
			So(output.Fragile, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given stronger depth collapse alongside moderate spread deviation", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:       100,
			BidDepthRatio:   0.05,
			AskDepthRatio:   1,
			DensityRatio:    0.5,
			SpreadDeviation: 0.3,
		})

		So(err, ShouldBeNil)

		Convey("It should compare category scores on the same scale", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.Mechanical, ShouldBeGreaterThan, output.Fragile)
		})
	})

	Convey("Given support-side imbalance flipping against the position", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:          100,
			BidDepthRatio:      1,
			AskDepthRatio:      1,
			DensityRatio:       1,
			Imbalance:          -0.5,
			PriorImbalanceMean: 0.3,
		})

		So(err, ShouldBeNil)

		Convey("It should classify active reversal", func() {
			So(int(output.Category), ShouldEqual, 4)
			So(output.Reversal, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a stable book without decay signals", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:     100,
			BidDepthRatio: 1,
			AskDepthRatio: 1,
			DensityRatio:  1,
		})

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

	Convey("Given the reflexive first-tick boundary where nothing precedes the reading", testingTB, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:     100,
			BidDepthRatio: 1,
			AskDepthRatio: 1,
			DensityRatio:  1,
			Pressure:      0.4,
			PressurePeak:  0.4,
		})

		So(err, ShouldBeNil)

		Convey("It should report no fade against a peak this tick just set", func() {
			So(output.Thermal, ShouldEqual, 0)
			So(int(output.Category), ShouldEqual, 0)
		})
	})
}

func BenchmarkDecayMeasure(benchmark *testing.B) {
	decay := equation.NewDecay()
	input := equation.DecayInput{
		LastPrice:       100,
		BidDepthRatio:   0.4,
		AskDepthRatio:   1,
		DensityRatio:    0.8,
		SpreadDeviation: 0.5,
		Pressure:        0.2,
		PressurePeak:    0.6,
		PressureTrough:  -0.1,
		Imbalance:       0.1,
	}

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = decay.Measure(input)
	}
}
