package flow

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestBook_Imbalance(testingTB *testing.T) {
	Convey("Given symmetric depth on a high-resolution integer price lattice", testingTB, func() {
		book := NewBook()
		tickSize := 0.00000001
		midTick := int64(10_000_000_000)
		So(book.Configure(BookInput{TickSize: tickSize}), ShouldBeNil)
		_, err := book.ApplyLevels([]BookLevel{
			{Price: TickPrice(midTick-1, tickSize), Ticks: midTick - 1, Quantity: 5},
			{Price: TickPrice(midTick-2, tickSize), Ticks: midTick - 2, Quantity: 3},
		}, SideBid)
		So(err, ShouldBeNil)
		_, err = book.ApplyLevels([]BookLevel{
			{Price: TickPrice(midTick+1, tickSize), Ticks: midTick + 1, Quantity: 5},
			{Price: TickPrice(midTick+2, tickSize), Ticks: midTick + 2, Quantity: 3},
		}, SideAsk)
		So(err, ShouldBeNil)

		Convey("It should preserve exact balance instead of fabricating float skew", func() {
			mid := TickPrice(midTick, tickSize)
			spread := 2 * tickSize
			So(book.Imbalance(mid, DecayRate(mid, spread), false, 0, 0, 0),
				ShouldEqual, 0)
		})
	})
}

/*
TestBook_Configure proves decimal→float tick restamps are lattice-stable and a
true increment change rebuilds an empty book instead of failing the hot path.
*/
func TestBook_Configure(testingTB *testing.T) {
	Convey("Given a book locked to a one-cent lattice", testingTB, func() {
		book := NewBook()
		So(book.Configure(BookInput{TickSize: 0.01}), ShouldBeNil)
		_, err := book.ApplyLevels([]BookLevel{
			{Price: 100.00, Ticks: 10000, Quantity: 1},
		}, SideBid)
		So(err, ShouldBeNil)

		Convey("It should accept a float-restamped equivalent tick", func() {
			restamped := 1.0 / 100.0
			So(restamped == 0.01 || sameTickSize(0.01, restamped), ShouldBeTrue)
			So(book.Configure(BookInput{TickSize: restamped}), ShouldBeNil)
			So(book.bids.Len(), ShouldEqual, 1)
		})

		Convey("It should clear levels when the lattice truly changes", func() {
			So(book.Configure(BookInput{TickSize: 0.1}), ShouldBeNil)
			So(book.tickSize, ShouldEqual, 0.1)
			So(book.bids.Len(), ShouldEqual, 0)
			So(book.asks.Len(), ShouldEqual, 0)
		})
	})
}

func TestBookflowMeasure(testingTB *testing.T) {
	Convey("Given a bid-heavy book snapshot", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(bookflowMeasureInput(
			0.85, 0.80, 0.86, true, 0.8,
		))

		So(err, ShouldBeNil)

		Convey("It should classify loaded imbalance", func() {
			So(output.Value, ShouldBeGreaterThan, 0)
			So(int(output.Category), ShouldEqual, 1)
		})
	})

	Convey("Given deep bid wall with bearish touch", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(equation.BookflowInput{
			Weighted:        0.6,
			Level1:          -0.4,
			Flat:            0.5,
			FlatOK:          true,
			Mid:             50,
			Spread:          2,
			TouchDepth:      3,
			TradePressure:   -0.5,
			WeightedHistory: []float64{0.6, 0.55, 0.58, 0.62},
			Level1History:   []float64{0.2, 0.18, 0.22, 0.19},
			FlatHistory:     []float64{0.25, 0.24, 0.26, 0.23},
		})

		So(err, ShouldBeNil)

		Convey("It should classify spoof trap", func() {
			So(int(output.Category), ShouldEqual, 2)
		})
	})
}

func BenchmarkBookflowMeasure(benchmark *testing.B) {
	bookflow := equation.NewBookflow()
	input := bookflowMeasureInput(0.85, 0.80, 0.86, true, 0.8)

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = bookflow.Measure(input)
	}
}

func bookflowMeasureInput(
	weighted float64,
	level1 float64,
	flat float64,
	flatOK bool,
	tradePressure float64,
) equation.BookflowInput {
	return equation.BookflowInput{
		Weighted:        weighted,
		Level1:          level1,
		Flat:            flat,
		FlatOK:          flatOK,
		Mid:             100,
		Spread:          2,
		TouchDepth:      12,
		TradePressure:   tradePressure,
		WeightedHistory: []float64{0.80, 0.82, 0.84, 0.86},
		Level1History:   []float64{0.78, 0.79, 0.80, 0.81},
		FlatHistory:     []float64{0.80, 0.82, 0.83, 0.84},
	}
}
