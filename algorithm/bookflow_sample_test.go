package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestBookflowSample_MeasureBook(testingTB *testing.T) {
	Convey("Given repeated bid-heavy book frames", testingTB, func() {
		sample := NewBookflowSample()
		bookflow := equation.NewBookflow()
		classifier := probability.NewScoreClassifier(
			[]string{"loadedScore", "spoofScore", "thinScore", "neutralScore"},
			nil,
		)

		var (
			input equation.BookflowInput
			ok    bool
			err   error
		)

		for range 6 {
			input, ok, err = sample.MeasureBook(bookflowBookInput())
		}

		So(err, ShouldBeNil)

		output, err := bookflow.Measure(input)
		So(err, ShouldBeNil)

		result, err := classifier.Classify(map[string]float64{
			"loadedScore":  output.LoadedScore,
			"spoofScore":   output.SpoofScore,
			"thinScore":    output.ThinScore,
			"neutralScore": output.NeutralScore,
			"strength":     output.Strength,
		})
		So(err, ShouldBeNil)

		Convey("It should emit calibrated depth-flow output", func() {
			So(ok, ShouldBeTrue)
			So(output.Ready, ShouldBeTrue)
			So(output.Value, ShouldBeGreaterThan, 0)
			So(result.Confidence, ShouldBeGreaterThan, 0.25)
		})
	})

	Convey("Given a valid first book frame before feature history is ready", testingTB, func() {
		sample := NewBookflowSample()
		input, ok, err := sample.MeasureBook(bookflowBookInput())

		Convey("It should publish a feature sample from the first valid book frame", func() {
			So(err, ShouldBeNil)
			So(ok, ShouldBeTrue)
			So(input.Mid, ShouldBeGreaterThan, 0)
			So(input.TouchDepth, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a book frame without symbol", testingTB, func() {
		sample := NewBookflowSample()
		_, _, err := sample.MeasureBook(BookflowBookInput{
			Bids: []BookLevel{{Price: 100, Quantity: 10}},
			Asks: []BookLevel{{Price: 101, Quantity: 10}},
		})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestBookflowSample_MeasureTrade(testingTB *testing.T) {
	Convey("Given trade pressure after book state", testingTB, func() {
		sample := NewBookflowSample()
		_, _, err := sample.MeasureBook(bookflowBookInput())
		So(err, ShouldBeNil)

		input, ok, err := sample.MeasureTrade(BookflowTradeInput{
			Symbol:   "BTC/USD",
			Side:     "buy",
			Price:    100,
			Quantity: 2,
		})

		Convey("It should update the sampled trade pressure", func() {
			So(err, ShouldBeNil)
			So(ok, ShouldBeTrue)
			So(input.TradePressure, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkBookflowSampleMeasureBook(benchmark *testing.B) {
	sample := NewBookflowSample()
	input := bookflowBookInput()

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _, _ = sample.MeasureBook(input)
	}
}

func bookflowBookInput() BookflowBookInput {
	return BookflowBookInput{
		Symbol: "BTC/USD",
		Bids: []BookLevel{
			{Price: 100, Quantity: 20},
			{Price: 99, Quantity: 18},
		},
		Asks: []BookLevel{
			{Price: 101, Quantity: 8},
			{Price: 102, Quantity: 6},
		},
	}
}
