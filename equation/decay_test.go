package equation_test

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestDecay_Measure(t *testing.T) {
	Convey("Given a bid depth ratio well below its own baseline", t, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:     100,
			BidDepthRatio: 0.3,
			AskDepthRatio: 1,
			DensityRatio:  1,
		})

		So(err, ShouldBeNil)

		Convey("It should classify mechanical exhaustion", func() {
			So(int(output.Long.Category), ShouldEqual, 1)
			So(output.Long.Value, ShouldBeGreaterThan, 0)
			So(output.Long.Urgency, ShouldEqual, output.Long.Value)
			So(output.Short.Urgency, ShouldEqual, 0)
		})
	})

	Convey("Given trade pressure well below its running peak", t, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:      100,
			PriceReturn:    -0.02,
			BidDepthRatio:  1,
			AskDepthRatio:  1,
			DensityRatio:   1,
			Pressure:       -0.1,
			PressurePeak:   0.9,
			PressureTrough: -0.1,
		})

		So(err, ShouldBeNil)

		Convey("It should classify thermal exhaustion", func() {
			So(int(output.Long.Category), ShouldEqual, 3)
			So(output.Long.Thermal, ShouldBeGreaterThan, 0)
			So(output.Short.Thermal, ShouldEqual, 0)
		})
	})

	Convey("Given pressure fade without an adverse price move", t, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:      100,
			PriceReturn:    0.02,
			BidDepthRatio:  1,
			AskDepthRatio:  1,
			DensityRatio:   1,
			Pressure:       -0.1,
			PressurePeak:   0.9,
			PressureTrough: -0.1,
		})

		So(err, ShouldBeNil)

		Convey("It should not call supportive price continuation exhaustion", func() {
			So(output.Long.Thermal, ShouldEqual, 0)
			So(output.Short.Thermal, ShouldEqual, 0)
		})
	})

	Convey("Given a spread deviation without depth collapse", t, func() {
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
			So(int(output.Long.Category), ShouldEqual, 2)
			So(output.Long.Fragile, ShouldBeGreaterThan, 0)
			So(output.Short.Fragile, ShouldEqual, output.Long.Fragile)
		})
	})

	Convey("Given stronger depth collapse alongside moderate spread deviation", t, func() {
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
			So(int(output.Long.Category), ShouldEqual, 1)
			So(output.Long.Mechanical, ShouldBeGreaterThan, output.Long.Fragile)
		})
	})

	Convey("Given support-side imbalance flipping against the position", t, func() {
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
			So(int(output.Long.Category), ShouldEqual, 4)
			So(output.Long.Reversal, ShouldBeGreaterThan, 0)
			So(output.Short.Reversal, ShouldEqual, 0)
		})
	})

	Convey("Given a stable book without decay signals", t, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:     100,
			BidDepthRatio: 1,
			AskDepthRatio: 1,
			DensityRatio:  1,
		})

		So(err, ShouldBeNil)

		Convey("It should emit zero urgency without rejecting the stage", func() {
			for _, side := range []equation.DecaySideOutput{output.Long, output.Short} {
				So(int(side.Category), ShouldEqual, 0)
				So(side.Urgency, ShouldEqual, 0)
				So(side.Mechanical, ShouldEqual, 0)
				So(side.Fragile, ShouldEqual, 0)
				So(side.Thermal, ShouldEqual, 0)
				So(side.Reversal, ShouldEqual, 0)
			}
		})
	})

	Convey("Given the reflexive first-tick boundary where nothing precedes the reading", t, func() {
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
			So(output.Long.Thermal, ShouldEqual, 0)
			So(int(output.Long.Category), ShouldEqual, 0)
		})
	})

	Convey("Given incomplete or non-finite decay inputs", t, func() {
		decay := equation.NewDecay()
		invalid := []equation.DecayInput{
			{LastPrice: 100},
			{
				LastPrice:     100,
				PriceReturn:   math.NaN(),
				BidDepthRatio: 1,
				AskDepthRatio: 1,
				DensityRatio:  1,
			},
		}

		Convey("It should reject missing ratios and non-finite evidence", func() {
			for _, input := range invalid {
				_, err := decay.Measure(input)
				So(err, ShouldNotBeNil)
			}
		})
	})
}

func BenchmarkDecayMeasure(b *testing.B) {
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

	b.ReportAllocs()

	for b.Loop() {
		_, _ = decay.Measure(input)
	}
}
