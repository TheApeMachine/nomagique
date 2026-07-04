package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func bookQualitySampleConfig() BookQualitySampleConfig {
	return DefaultBookQualitySampleConfig()
}

func TestNewBookQualitySample(t *testing.T) {
	Convey("Given a book quality sample stage", t, func() {
		stage := NewBookQualitySample(bookQualitySampleConfig())

		Convey("It should be constructible", func() {
			So(stage, ShouldNotBeNil)
		})
	})
}

func TestBookQualitySample_MeasureBook(t *testing.T) {
	Convey("Given an aggregate L2 book frame", t, func() {
		sample := NewBookQualitySample(bookQualitySampleConfig())
		input, ready, err := sample.MeasureBook(BookflowBookInput{
			Symbol: "BTC/USD",
			Bids: []BookLevel{
				{Price: 100, Quantity: 10},
			},
			Asks: []BookLevel{
				{Price: 101, Quantity: 10},
			},
		})

		Convey("It should update depth without claiming L3 toxicity", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(input.LastPrice, ShouldEqual, 100.5)
			So(input.ToxicNear, ShouldBeFalse)
			So(input.BidDepth, ShouldEqual, 10)
			So(input.AskDepth, ShouldEqual, 10)
		})
	})
}

func TestBookQualitySample_MeasureLevel3(t *testing.T) {
	Convey("Given level3 frames for multiple symbols", t, func() {
		sample := NewBookQualitySample(bookQualitySampleConfig())

		btc, btcReady, btcErr := sample.MeasureLevel3(BookQualityLevel3Input{
			Symbol: "BTC/USD",
			Bids: []BookQualityOrderEvent{
				{Event: "add", OrderID: "B1", Price: 100, Quantity: 20},
			},
			Asks: []BookQualityOrderEvent{
				{Event: "add", OrderID: "A1", Price: 101, Quantity: 20},
			},
		})

		Convey("It should emit L3 inputs for the first symbol", func() {
			So(btcErr, ShouldBeNil)
			So(btcReady, ShouldBeTrue)
			So(btc.LastPrice, ShouldEqual, 100.5)
		})

		eth, ethReady, ethErr := sample.MeasureLevel3(BookQualityLevel3Input{
			Symbol: "ETH/USD",
			Bids: []BookQualityOrderEvent{
				{Event: "add", OrderID: "B1", Price: 200, Quantity: 20},
			},
			Asks: []BookQualityOrderEvent{
				{Event: "add", OrderID: "A1", Price: 201, Quantity: 20},
			},
		})

		Convey("It should keep per-symbol books isolated", func() {
			So(ethErr, ShouldBeNil)
			So(ethReady, ShouldBeTrue)
			So(eth.LastPrice, ShouldEqual, 200.5)
		})
	})
}

func BenchmarkBookQualitySample_MeasureLevel3(b *testing.B) {
	sample := NewBookQualitySample(bookQualitySampleConfig())
	input := BookQualityLevel3Input{
		Symbol: "BTC/USD",
		Bids: []BookQualityOrderEvent{
			{Event: "add", OrderID: "B1", Price: 100, Quantity: 10},
		},
		Asks: []BookQualityOrderEvent{
			{Event: "add", OrderID: "A1", Price: 101, Quantity: 10},
		},
	}

	b.ReportAllocs()

	for b.Loop() {
		_, _, _ = sample.MeasureLevel3(input)
	}
}
