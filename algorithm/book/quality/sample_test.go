package quality

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
)

func sampleConfig() SampleConfig {
	return DefaultSampleConfig()
}

func TestNewSample(t *testing.T) {
	Convey("Given a book quality sample stage", t, func() {
		stage := NewSample(sampleConfig())

		Convey("It should be constructible", func() {
			So(stage, ShouldNotBeNil)
		})
	})
}

func TestSample_MeasureBook(t *testing.T) {
	Convey("Given an aggregate L2 book frame", t, func() {
		sample := NewSample(sampleConfig())
		input, ready, err := sample.MeasureBook(flow.BookInput{
			Symbol: "BTC/USD",
			Bids: []flow.BookLevel{
				{Price: 100, Quantity: 10},
			},
			Asks: []flow.BookLevel{
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

func TestSample_MeasureLevel3(t *testing.T) {
	Convey("Given level3 frames for multiple symbols", t, func() {
		sample := NewSample(sampleConfig())

		btc, btcReady, btcErr := sample.MeasureLevel3(Level3Input{
			Symbol: "BTC/USD",
			Bids: []OrderEvent{
				{Event: "add", OrderID: "B1", Price: 100, Quantity: 20},
			},
			Asks: []OrderEvent{
				{Event: "add", OrderID: "A1", Price: 101, Quantity: 20},
			},
		})

		Convey("It should emit L3 inputs for the first symbol", func() {
			So(btcErr, ShouldBeNil)
			So(btcReady, ShouldBeTrue)
			So(btc.LastPrice, ShouldEqual, 100.5)
		})

		eth, ethReady, ethErr := sample.MeasureLevel3(Level3Input{
			Symbol: "ETH/USD",
			Bids: []OrderEvent{
				{Event: "add", OrderID: "B1", Price: 200, Quantity: 20},
			},
			Asks: []OrderEvent{
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

func BenchmarkSample_MeasureLevel3(b *testing.B) {
	sample := NewSample(sampleConfig())
	input := Level3Input{
		Symbol: "BTC/USD",
		Bids: []OrderEvent{
			{Event: "add", OrderID: "B1", Price: 100, Quantity: 10},
		},
		Asks: []OrderEvent{
			{Event: "add", OrderID: "A1", Price: 101, Quantity: 10},
		},
	}

	b.ReportAllocs()

	for b.Loop() {
		_, _, _ = sample.MeasureLevel3(input)
	}
}
