package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestBookQualityToxicBluff(testingTB *testing.T) {
	Convey("Given near-touch toxic churn above gate", testingTB, func() {
		bookQuality := equation.NewBookQuality()
		output, err := bookQuality.Measure(equation.BookQualityInput{
			FillBid:            0.1,
			FillAsk:            0.1,
			BidDepth:           80,
			AskDepth:           80,
			ToxicNear:          true,
			ToxicBluffStrength: 4.5,
			Threshold:          0.15,
			ChurnGate:          0.8,
			VacuumStrengthCap:  2,
			LastPrice:          100,
		})

		So(err, ShouldBeNil)

		Convey("It should classify toxic bluff", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.Value, ShouldEqual, 4.5)
		})
	})
}

func TestBookQualityLiquidityVacuum(testingTB *testing.T) {
	Convey("Given cancel/fill asymmetry with fill flow", testingTB, func() {
		bookQuality := equation.NewBookQuality()
		output, err := bookQuality.Measure(equation.BookQualityInput{
			CancelBid:         0.3,
			FillBid:           0.1,
			BidDepth:          10,
			AskDepth:          10,
			Threshold:         0.15,
			VacuumStrengthCap: 2,
			LastPrice:         50000,
		})

		So(err, ShouldBeNil)

		Convey("It should classify liquidity vacuum", func() {
			So(int(output.Category), ShouldEqual, 2)
			So(output.Value, ShouldBeGreaterThan, 0)
		})
	})
}

func TestBookQualityHardSupport(testingTB *testing.T) {
	Convey("Given balanced depth with fills and no cancels", testingTB, func() {
		bookQuality := equation.NewBookQuality()
		output, err := bookQuality.Measure(equation.BookQualityInput{
			FillBid:           0.1,
			FillAsk:           0.1,
			BidDepth:          80,
			AskDepth:          80,
			Threshold:         0.15,
			VacuumStrengthCap: 1,
			LastPrice:         100,
		})

		So(err, ShouldBeNil)

		Convey("It should classify hard support", func() {
			So(int(output.Category), ShouldEqual, 3)
			So(output.Value, ShouldEqual, 1)
		})
	})
}

func BenchmarkBookQualityMeasure(benchmark *testing.B) {
	bookQuality := equation.NewBookQuality()
	input := equation.BookQualityInput{
		CancelBid:         0.3,
		FillBid:           0.1,
		BidDepth:          10,
		AskDepth:          10,
		Threshold:         0.15,
		VacuumStrengthCap: 2,
		LastPrice:         50000,
	}

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = bookQuality.Measure(input)
	}
}
