package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
)

func TestTradeExcitationSample_MeasureTrade(testingTB *testing.T) {
	Convey("Given one trade before excitation history is warm", testingTB, func() {
		sample := NewTradeExcitationSample()
		input, ready, err := sample.MeasureTrade(tradeExcitationInput(
			"ALT/EUR",
			"buy",
			time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		))

		Convey("It should stage without publishing features", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeFalse)
			So(input.Symbol, ShouldEqual, "")
		})
	})

	Convey("Given alternating buy and sell trades", testingTB, func() {
		sample := NewTradeExcitationSample()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

		var last ExcitationInput
		var ready bool
		var err error

		for index := range 64 {
			side := "buy"

			if index%2 == 0 {
				side = "sell"
			}

			last, ready, err = sample.MeasureTrade(
				tradeExcitationInput(
					"ALT/EUR",
					side,
					base.Add(time.Duration(index)*100*time.Millisecond),
				),
			)
		}

		Convey("It should publish an excitation feature batch", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(last.Symbol, ShouldEqual, "ALT/EUR")
			So(len(last.BuySeconds), ShouldBeGreaterThan, 0)
			So(len(last.SellSeconds), ShouldBeGreaterThan, 0)
		})
	})
}

func TestTradeExcitationSample_MeasureBook(testingTB *testing.T) {
	Convey("Given a book frame before a warmed trade sample", testingTB, func() {
		sample := NewTradeExcitationSample()
		err := sample.MeasureBook(bookTouchInput("ALT/EUR", 1000, 200))
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

		var last ExcitationInput

		for index := range 64 {
			side := "buy"

			if index%2 == 0 {
				side = "sell"
			}

			last, _, _ = sample.MeasureTrade(
				tradeExcitationInput(
					"ALT/EUR",
					side,
					base.Add(time.Duration(index)*100*time.Millisecond),
				),
			)
		}

		Convey("It should include touch imbalance in the feature batch", func() {
			So(err, ShouldBeNil)
			So(last.TouchImbalance, ShouldAlmostEqual, 2.0/3.0, 0.001)
		})
	})
}

func BenchmarkTradeExcitationSample_MeasureTrade(b *testing.B) {
	sample := NewTradeExcitationSample()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

	b.ReportAllocs()

	for b.Loop() {
		_, _, _ = sample.MeasureTrade(
			tradeExcitationInput("ALT/EUR", "buy", base.Add(time.Duration(b.N)*time.Millisecond)),
		)
	}
}

func tradeExcitationInput(symbol string, side string, at time.Time) TradeExcitationInput {
	return TradeExcitationInput{
		Symbol:    symbol,
		Side:      side,
		Timestamp: at,
	}
}

func bookTouchInput(symbol string, bidQty, askQty float64) flow.BookInput {
	return flow.BookInput{
		Symbol: symbol,
		Bids: []flow.BookLevel{
			{Price: 1, Quantity: bidQty},
		},
		Asks: []flow.BookLevel{
			{Price: 1.01, Quantity: askQty},
		},
	}
}
