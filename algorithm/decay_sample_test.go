package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestDecaySample_MeasureBook(testingTB *testing.T) {
	Convey("Given deteriorating bid depth on repeated book frames", testingTB, func() {
		sample := NewDecaySample()
		decay := equation.NewDecay()
		classifier := probability.NewScoreClassifier(
			[]string{"mechanical", "fragile", "thermal", "reversal"},
			nil,
		)
		quantities := []float64{20, 18, 16, 14, 12, 10, 8, 6, 4}

		var (
			input equation.DecayInput
			ok    bool
			err   error
		)

		for _, bidQuantity := range quantities {
			input, ok, err = sample.MeasureBook(decayBookInput(bidQuantity, 10))
		}

		So(err, ShouldBeNil)

		output, err := decay.Measure(input)
		So(err, ShouldBeNil)

		result, err := classifier.Classify(map[string]float64{
			"mechanical": output.Mechanical,
			"fragile":    output.Fragile,
			"thermal":    output.Thermal,
			"reversal":   output.Reversal,
			"strength":   output.Strength,
		})
		So(err, ShouldBeNil)

		Convey("It should emit calibrated decay output", func() {
			So(ok, ShouldBeTrue)
			So(output.Value, ShouldBeGreaterThan, 0)
			So(result.Confidence, ShouldBeGreaterThan, 0.25)
		})
	})

	Convey("Given stable bid depth on repeated book frames", testingTB, func() {
		sample := NewDecaySample()

		var err error
		for range 12 {
			_, _, err = sample.MeasureBook(decayBookInput(10, 10))
		}

		Convey("It should complete sampling after history accumulates", func() {
			So(err, ShouldBeNil)
		})
	})

	Convey("Given a valid first book frame before feature history is ready", testingTB, func() {
		sample := NewDecaySample()
		input, ok, err := sample.MeasureBook(decayBookInput(10, 10))

		Convey("It should be a nonfatal not-ready sample", func() {
			So(err, ShouldBeNil)
			So(ok, ShouldBeFalse)
			So(input.LastPrice, ShouldEqual, 0)
		})
	})

	Convey("Given a book frame without symbol", testingTB, func() {
		sample := NewDecaySample()
		_, _, err := sample.MeasureBook(flow.BookInput{
			Bids: []flow.BookLevel{{Price: 100, Quantity: 10}},
			Asks: []flow.BookLevel{{Price: 101, Quantity: 10}},
		})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkDecaySampleMeasureBook(benchmark *testing.B) {
	sample := NewDecaySample()

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _, _ = sample.MeasureBook(decayBookInput(10, 10))
	}
}

func decayBookInput(bidQuantity float64, askQuantity float64) flow.BookInput {
	tickSize := 1.0

	return flow.BookInput{
		Symbol:   "BTC/USD",
		TickSize: tickSize,
		Bids: []flow.BookLevel{
			{Price: 100, Ticks: 100, Quantity: bidQuantity},
		},
		Asks: []flow.BookLevel{
			{Price: 101, Ticks: 101, Quantity: askQuantity},
		},
	}
}
