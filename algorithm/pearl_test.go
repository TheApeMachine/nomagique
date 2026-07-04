package algorithm_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/algorithm"
)

func pearlConfig() algorithm.PearlConfig {
	return algorithm.PearlConfig{
		MinHistory:      5,
		History:         16,
		CategoryIndexes: []float64{1, 2, 3, 4},
	}
}

func TestPearl_MeasureTrade(testingTB *testing.T) {
	Convey("Given local flow that builds price association", testingTB, func() {
		pearl := algorithm.NewPearl(pearlConfig())
		price := 100.0
		var output algorithm.PearlOutput
		var ready bool
		var err error

		for index := range 12 {
			flow := 1 + float64(index)
			price *= 1 + flow*0.001
			output, ready, err = pearl.MeasureTrade(algorithm.PearlTradeInput{
				Symbol:   "BTC/USD",
				Price:    price,
				Quantity: flow,
				Side:     "buy",
			})
		}

		output, ready, err = pearl.MeasureTrade(algorithm.PearlTradeInput{
			Symbol:   "BTC/USD",
			Price:    price,
			Quantity: 1,
			Side:     "buy",
		})

		Convey("It emits counterfactual alpha from direct numeric inputs", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(output.AlphaScore, ShouldBeGreaterThan, 0)
			So(output.UpliftScore, ShouldBeGreaterThan, 0)
			So(output.Category, ShouldEqual, 1)
		})
	})
}

func TestPearl_MeasureBook(testingTB *testing.T) {
	Convey("Given liquidity stress that outruns its own history", testingTB, func() {
		pearl := algorithm.NewPearl(pearlConfig())
		var output algorithm.PearlOutput
		var ready bool
		var err error

		for index := range 8 {
			price := 100 + float64(index)*0.01
			depth := 100 - float64(index)
			_, _, err = pearl.MeasureTicker(algorithm.PearlTickerInput{
				Symbol:    "BTC/USD",
				Last:      price,
				ChangePct: 0.01,
				Bid:       price - 0.01,
				Ask:       price + 0.01,
				BidQty:    depth,
				AskQty:    depth,
			})
			So(err, ShouldBeNil)
		}

		output, ready, err = pearl.MeasureBook(algorithm.PearlBookInput{
			Symbol: "BTC/USD",
			Bids:   []algorithm.BookLevel{{Price: 90, Quantity: 0.01}},
			Asks:   []algorithm.BookLevel{{Price: 110, Quantity: 0.01}},
		})

		Convey("It emits liquidity shock without artifact round-tripping", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(output.ShockScore, ShouldBeGreaterThan, 0)
			So(output.Inverted, ShouldBeTrue)
			So(output.Category, ShouldEqual, 3)
		})
	})
}

func TestPearlSample_MeasureTicker(testingTB *testing.T) {
	Convey("Given ticker rows for two symbols", testingTB, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		var btc algorithm.PearlSampleOutput

		for index := range 6 {
			var ready bool
			var err error
			btc, ready, err = sample.MeasureTicker(algorithm.PearlTickerInput{
				Symbol:    "BTC/USD",
				Last:      100 + float64(index),
				ChangePct: 0.01 * float64(index),
				Bid:       99 + float64(index),
				Ask:       101 + float64(index),
				BidQty:    20,
				AskQty:    18,
			})

			So(err, ShouldBeNil)
			So(ready, ShouldEqual, index >= 4)
		}

		eth, ready, err := sample.MeasureTicker(algorithm.PearlTickerInput{
			Symbol:    "ETH/USD",
			Last:      50,
			ChangePct: 0.02,
			Bid:       49,
			Ask:       51,
			BidQty:    9,
			AskQty:    8,
		})

		Convey("It keeps per-symbol rolling rows separate", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeFalse)
			So(btc.Rows, ShouldHaveLength, 6)
			So(eth.Rows, ShouldHaveLength, 1)
			So(btc.Row[0], ShouldEqual, 0.0005)
			So(eth.Row[0], ShouldEqual, 0.0002)
		})
	})
}

func TestPearlSample_MeasureBook(testingTB *testing.T) {
	Convey("Given a book row", testingTB, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		output, ready, err := sample.MeasureBook(algorithm.PearlBookInput{
			Symbol: "BTC/USD",
			Bids: []algorithm.BookLevel{
				{Price: 100, Quantity: 4},
				{Price: 99, Quantity: 10},
			},
			Asks: []algorithm.BookLevel{
				{Price: 101, Quantity: 3},
				{Price: 102, Quantity: 10},
			},
		})

		Convey("It encodes liquidity stress into the causal row", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeFalse)
			So(output.Row, ShouldHaveLength, 4)
			So(output.Row[1], ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkPearl_MeasureTrade(testingTB *testing.B) {
	pearl := algorithm.NewPearl(pearlConfig())
	price := 100.0

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		for index := range 12 {
			flow := 1 + float64(index)
			price *= 1 + flow*0.001
			_, _, _ = pearl.MeasureTrade(algorithm.PearlTradeInput{
				Symbol:   "BTC/USD",
				Price:    price,
				Quantity: flow,
				Side:     "buy",
			})
		}
	}
}
