package flow

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestBookflowSample_MeasureBook(testingTB *testing.T) {
	Convey("Given repeated bid-heavy book frames", testingTB, func() {
		sample := NewSample()
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
			input, ok, _, err = sample.MeasureBook(bookflowBookInput())
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
		sample := NewSample()
		input, ok, _, err := sample.MeasureBook(bookflowBookInput())

		Convey("It should publish a feature sample from the first valid book frame", func() {
			So(err, ShouldBeNil)
			So(ok, ShouldBeTrue)
			So(input.Mid, ShouldBeGreaterThan, 0)
			So(input.TouchDepth, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given repeated prices that resolve to the same exchange tick", testingTB, func() {
		sample := NewSample()
		price := 100.1
		tickSize := 0.1

		_, _, _, err := sample.MeasureBook(BookInput{
			Symbol:   "BTC/USD",
			TickSize: tickSize,
			Bids: []BookLevel{
				{Price: price, Ticks: 1001, Quantity: 20},
			},
			Asks: []BookLevel{
				{Price: 100.3, Ticks: 1003, Quantity: 8},
			},
		})
		So(err, ShouldBeNil)

		_, _, _, err = sample.MeasureBook(BookInput{
			Symbol:   "BTC/USD",
			TickSize: tickSize,
			Bids: []BookLevel{
				{Price: 100.100000000002, Ticks: 1001, Quantity: 18},
			},
		})
		So(err, ShouldBeNil)

		windowValue, found := sample.windows.Load("BTC/USD")
		So(found, ShouldBeTrue)
		window := windowValue.(*Window)

		Convey("It should update the existing integer price tick", func() {
			So(window.book.bids.Len(), ShouldEqual, 1)

			qtyValue, qtyFound := window.book.bids.levels.Load(int64(1001))
			So(qtyFound, ShouldBeTrue)
			So(qtyValue.(float64), ShouldEqual, 18)
		})
	})

	Convey("Given a book frame without symbol", testingTB, func() {
		sample := NewSample()
		_, _, _, err := sample.MeasureBook(BookInput{
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
		sample := NewSample()
		_, _, _, err := sample.MeasureBook(bookflowBookInput())
		So(err, ShouldBeNil)

		input, ok, _, err := sample.MeasureTrade(TradeInput{
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
	sample := NewSample()
	input := bookflowBookInput()

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _, _, _ = sample.MeasureBook(input)
	}
}

func bookflowBookInput() BookInput {
	tickSize := 1.0

	return BookInput{
		Symbol:   "BTC/USD",
		TickSize: tickSize,
		Bids: []BookLevel{
			bookflowTestLevel(100, tickSize, 20),
			bookflowTestLevel(99, tickSize, 18),
		},
		Asks: []BookLevel{
			bookflowTestLevel(101, tickSize, 8),
			bookflowTestLevel(102, tickSize, 6),
		},
	}
}

func bookflowTestLevel(price float64, tickSize float64, quantity float64) BookLevel {
	return BookLevel{
		Price:    price,
		Ticks:    int64(price),
		Quantity: quantity,
	}
}
