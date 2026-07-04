package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestBookQuality_Measure(testingTB *testing.T) {
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

	Convey("Given balanced depth without category evidence", testingTB, func() {
		bookQuality := equation.NewBookQuality()
		output, err := bookQuality.Measure(equation.BookQualityInput{
			BidDepth:          80,
			AskDepth:          80,
			Threshold:         0.15,
			VacuumStrengthCap: 1,
			LastPrice:         100,
		})

		So(err, ShouldBeNil)

		Convey("It should emit uniform neutral confidence instead of rejecting", func() {
			classified := classifyBookQualityOutput(output)

			So(classified.Confidence, ShouldAlmostEqual, 1.0/3.0)
			So(classified.Strength, ShouldEqual, 0)
		})
	})

	Convey("Given cancel/fill evidence before the adaptive threshold is ready", testingTB, func() {
		bookQuality := equation.NewBookQuality()
		output, err := bookQuality.Measure(equation.BookQualityInput{
			CancelBid:         0.3,
			FillBid:           0.1,
			BidDepth:          80,
			AskDepth:          80,
			VacuumStrengthCap: 1,
			LastPrice:         100,
		})

		So(err, ShouldBeNil)

		Convey("It should remain neutral instead of rejecting threshold warmup", func() {
			classified := classifyBookQualityOutput(output)

			So(classified.Confidence, ShouldAlmostEqual, 1.0/3.0)
			So(classified.Strength, ShouldEqual, 0)
		})
	})

	Convey("Given liquidity vacuum evidence without a configured strength cap", testingTB, func() {
		bookQuality := equation.NewBookQuality()
		output, err := bookQuality.Measure(equation.BookQualityInput{
			CancelBid: 0.3,
			FillBid:   0.1,
			BidDepth:  80,
			AskDepth:  80,
			Threshold: 0.15,
			LastPrice: 100,
		})

		So(err, ShouldBeNil)

		Convey("It should use bounded vacuum evidence as strength", func() {
			So(int(output.Category), ShouldEqual, 2)
			So(output.Value, ShouldAlmostEqual, output.VacuumScore)
			So(output.Value, ShouldBeLessThan, 1)
		})
	})

	Convey("Given cancels without fills on one side", testingTB, func() {
		bookQuality := equation.NewBookQuality()
		output, err := bookQuality.Measure(equation.BookQualityInput{
			CancelAsk:         4,
			BidDepth:          80,
			AskDepth:          80,
			Threshold:         0.15,
			VacuumStrengthCap: 1,
			LastPrice:         100,
		})

		So(err, ShouldBeNil)

		Convey("It should remain neutral instead of inventing a ratio", func() {
			classified := classifyBookQualityOutput(output)

			So(classified.Confidence, ShouldAlmostEqual, 1.0/3.0)
			So(classified.Strength, ShouldEqual, 0)
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

func BenchmarkBookQualityMeasureNeutral(benchmark *testing.B) {
	bookQuality := equation.NewBookQuality()
	input := equation.BookQualityInput{
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

func classifyBookQualityOutput(output equation.BookQualityOutput) probability.ScoreResult {
	classifier := probability.NewScoreClassifier(
		[]string{"bluffScore", "vacuumScore", "supportScore"},
		nil,
	)
	result, err := classifier.Classify(map[string]float64{
		"bluffScore":   output.BluffScore,
		"vacuumScore":  output.VacuumScore,
		"supportScore": output.SupportScore,
		"strength":     output.Strength,
	})

	So(err, ShouldBeNil)

	return result
}
