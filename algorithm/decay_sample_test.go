package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestDecaySample_MeasureBook(t *testing.T) {
	Convey("Given deteriorating bid depth on repeated book frames", t, func() {
		sample := NewDecaySample()
		decay := equation.NewDecay()
		classifier := probability.NewScoreClassifier(
			[]string{"mechanical", "fragile", "thermal", "reversal"},
			nil,
		)
		quantities := []float64{20, 18, 16, 14, 12, 10, 8, 6, 4}

		var (
			input    equation.DecayInput
			ok       bool
			maturity float64
			err      error
		)

		for _, bidQuantity := range quantities {
			input, ok, maturity, err = sample.MeasureBook(decayBookInput(bidQuantity, 10))
		}

		So(err, ShouldBeNil)

		output, err := decay.Measure(input)
		So(err, ShouldBeNil)

		result, err := classifier.Classify(map[string]float64{
			"mechanical": output.Long.Mechanical,
			"fragile":    output.Long.Fragile,
			"thermal":    output.Long.Thermal,
			"reversal":   output.Long.Reversal,
			"strength":   output.Long.Strength,
		})
		So(err, ShouldBeNil)

		Convey("It should emit calibrated decay output with high maturity", func() {
			So(ok, ShouldBeTrue)
			So(output.Long.Value, ShouldBeGreaterThan, 0)
			So(result.Confidence, ShouldBeGreaterThan, 0.25)
			So(maturity, ShouldBeGreaterThan, 0.85)
			So(maturity, ShouldBeLessThan, 1)
		})
	})

	Convey("Given stable bid depth on repeated book frames", t, func() {
		sample := NewDecaySample()

		var err error
		for range 12 {
			_, _, _, err = sample.MeasureBook(decayBookInput(10, 10))
		}

		Convey("It should complete sampling after history accumulates", func() {
			So(err, ShouldBeNil)
		})
	})

	Convey("Given a valid first book frame", t, func() {
		sample := NewDecaySample()
		input, ok, maturity, err := sample.MeasureBook(decayBookInput(10, 10))

		Convey("It should emit immediately with low maturity, not suppress", func() {
			So(err, ShouldBeNil)
			So(ok, ShouldBeTrue)
			So(input.LastPrice, ShouldBeGreaterThan, 0)
			So(maturity, ShouldBeGreaterThan, 0)
			So(maturity, ShouldBeLessThan, 1)
		})
	})

	Convey("Given a book frame without symbol", t, func() {
		sample := NewDecaySample()
		_, _, _, err := sample.MeasureBook(flow.BookInput{
			Bids: []flow.BookLevel{{Price: 100, Quantity: 10}},
			Asks: []flow.BookLevel{{Price: 101, Quantity: 10}},
		})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a crossed book after a valid observation", t, func() {
		sample := NewDecaySample()
		_, ready, _, err := sample.MeasureBook(decayBookInput(10, 10))
		So(err, ShouldBeNil)
		So(ready, ShouldBeTrue)

		input, ready, maturity, err := sample.MeasureBook(flow.BookInput{
			Symbol:   "BTC/USD",
			TickSize: 1,
			Bids:     []flow.BookLevel{{Price: 101, Ticks: 101, Quantity: 10}},
			Asks:     []flow.BookLevel{{Price: 100, Ticks: 100, Quantity: 10}},
		})

		Convey("It should stay not-ready without inventing features or failing", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeFalse)
			So(maturity, ShouldEqual, 0)
			So(input, ShouldResemble, equation.DecayInput{})
		})
	})
}

/*
TestDecaySample_MeasureTrade proves trades update pressure immediately but do
not fabricate the missing book ratios required by exhaustion.
*/
func TestDecaySample_MeasureTrade(t *testing.T) {
	Convey("Given a valid trade before any book observation", t, func() {
		sample := NewDecaySample()
		input, ready, maturity, err := sample.MeasureTrade(flow.TradeInput{
			Symbol:   "BTC/USD",
			Price:    100,
			Quantity: 2,
			Side:     "buy",
		})

		Convey("It should retain pressure without scoring absent book inputs", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeFalse)
			So(maturity, ShouldEqual, 0)
			So(input, ShouldResemble, equation.DecayInput{})
		})
	})

	Convey("Given a trade after a valid book observation", t, func() {
		sample := NewDecaySample()
		_, ready, _, err := sample.MeasureBook(decayBookInput(10, 10))
		So(err, ShouldBeNil)
		So(ready, ShouldBeTrue)

		input, ready, maturity, err := sample.MeasureTrade(flow.TradeInput{
			Symbol:   "BTC/USD",
			Price:    100,
			Quantity: 2,
			Side:     "buy",
		})

		Convey("It should expose the trade pressure on that trade event", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(input.Pressure, ShouldBeGreaterThan, 0)
			So(input.PressurePeak, ShouldEqual, input.Pressure)
			So(maturity, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a trade without a Kraken aggressor side", t, func() {
		sample := NewDecaySample()
		_, _, _, err := sample.MeasureTrade(flow.TradeInput{
			Symbol:   "BTC/USD",
			Price:    100,
			Quantity: 2,
		})

		Convey("It should reject the ambiguous pressure direction", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkDecaySampleMeasureBook(b *testing.B) {
	sample := NewDecaySample()

	b.ReportAllocs()

	for b.Loop() {
		_, _, _, _ = sample.MeasureBook(decayBookInput(10, 10))
	}
}

/*
BenchmarkDecaySampleMeasureTrade measures immediate pressure updates after a
book has established the required microstructure state.
*/
func BenchmarkDecaySampleMeasureTrade(b *testing.B) {
	sample := NewDecaySample()
	_, _, _, _ = sample.MeasureBook(decayBookInput(10, 10))
	input := flow.TradeInput{
		Symbol:   "BTC/USD",
		Price:    100,
		Quantity: 2,
		Side:     "buy",
	}

	b.ReportAllocs()

	for b.Loop() {
		_, _, _, _ = sample.MeasureTrade(input)
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
