package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestTradeFlowSample_Measure(testingTB *testing.T) {
	Convey("Given a sequence of buy trades", testingTB, func() {
		sample := NewTradeFlowSample()
		flow := equation.NewFlow()
		classifier := probability.NewScoreClassifier(
			[]string{"absorption", "drive", "balance", "starvation"},
			nil,
		)

		var (
			input equation.FlowInput
			ok    bool
			err   error
		)

		for index := range 30 {
			input, ok, err = sample.Measure(TradeFlowInput{
				Symbol:   "BTC/USD",
				Side:     "buy",
				Price:    100 + float64(index)*0.01,
				Quantity: 1,
			})
		}

		So(err, ShouldBeNil)

		output, err := flow.Measure(input)
		So(err, ShouldBeNil)

		result, err := classifier.Classify(map[string]float64{
			"absorption": output.Absorption,
			"drive":      output.Drive,
			"balance":    output.Balance,
			"starvation": output.Starvation,
			"strength":   output.Value,
		})
		So(err, ShouldBeNil)

		Convey("It should publish flow input after history warms", func() {
			So(ok, ShouldBeTrue)
			So(input.TradeCount, ShouldBeGreaterThan, 0)
			So(output.Drive, ShouldBeGreaterThan, 0)
			So(result.Confidence, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a trade without a symbol", testingTB, func() {
		sample := NewTradeFlowSample()
		_, _, err := sample.Measure(TradeFlowInput{
			Side:     "buy",
			Price:    100,
			Quantity: 1,
		})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a single valid trade", testingTB, func() {
		sample := NewTradeFlowSample()
		input, ok, err := sample.Measure(TradeFlowInput{
			Symbol:   "BTC/USD",
			Side:     "buy",
			Price:    100,
			Quantity: 1,
		})

		Convey("It should publish a feature sample from the first valid trade", func() {
			So(err, ShouldBeNil)
			So(ok, ShouldBeTrue)
			So(input.TradeCount, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkTradeFlowSampleMeasure(benchmark *testing.B) {
	sample := NewTradeFlowSample()

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _, _ = sample.Measure(TradeFlowInput{
			Symbol:   "BTC/USD",
			Side:     "buy",
			Price:    100,
			Quantity: 1,
		})
	}
}
