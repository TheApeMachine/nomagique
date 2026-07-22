package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestDecayMeasure(t *testing.T) {
	Convey("Given deteriorating long-side book history", t, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:     100,
			BidDepthRatio: 0.3,
			AskDepthRatio: 1,
			DensityRatio:  1,
		})

		So(err, ShouldBeNil)

		Convey("It should publish an eligible exhaustion outcome", func() {
			So(output.Long.Value, ShouldBeGreaterThan, 0)
			So(output.Long.Urgency, ShouldBeGreaterThan, 0)
			So(int(output.Long.Category), ShouldEqual, 1)
		})
	})

	Convey("Given pressure fade on the long side", t, func() {
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
		})
	})

	Convey("Given ask-side thinning stronger than bid-side", t, func() {
		decay := equation.NewDecay()
		output, err := decay.Measure(equation.DecayInput{
			LastPrice:     100,
			BidDepthRatio: 1,
			AskDepthRatio: 0.2,
			DensityRatio:  1,
		})

		So(err, ShouldBeNil)

		Convey("It should let the stronger short-side score win", func() {
			So(output.Short.Value, ShouldBeGreaterThan, 0)
			So(int(output.Short.Category), ShouldEqual, 1)
			So(output.Long.Value, ShouldEqual, 0)
		})
	})
}

func BenchmarkDecayMeasure(b *testing.B) {
	decay := equation.NewDecay()
	input := equation.DecayInput{
		LastPrice:     100,
		BidDepthRatio: 0.3,
		AskDepthRatio: 1,
		DensityRatio:  1,
	}

	b.ReportAllocs()

	for b.Loop() {
		_, _ = decay.Measure(input)
	}
}
