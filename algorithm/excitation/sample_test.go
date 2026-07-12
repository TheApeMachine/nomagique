package excitation

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSample_MeasureArrival(t *testing.T) {
	Convey("Given distinct trades one nanosecond apart", t, func() {
		sample := NewSample()
		base := time.Date(2026, 5, 30, 12, 0, 0, 1, time.UTC)
		_, _, err := sample.MeasureArrival(
			tradeInput("ALT/EUR", "buy", base),
		)
		input, ready, err := sample.MeasureArrival(
			tradeInput("ALT/EUR", "sell", base.Add(time.Nanosecond)),
		)

		Convey("It should preserve both native timestamps exactly", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(input.Stream.BuyTimes()[0].UnixNano(), ShouldEqual, base.UnixNano())
			So(input.Stream.SellTimes()[0].UnixNano(), ShouldEqual,
				base.Add(time.Nanosecond).UnixNano())
		})
	})

	Convey("Given an invalid trade side", t, func() {
		sample := NewSample()
		_, _, err := sample.MeasureArrival(tradeInput(
			"ALT/EUR", "unknown", time.Now(),
		))

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestTradeInput_ArrivalTime(t *testing.T) {
	Convey("Given an integer exchange timestamp", t, func() {
		expected := time.Date(2026, 5, 30, 12, 0, 0, 17, time.UTC)
		input := TradeInput{UnixNano: expected.UnixNano()}

		Convey("It should reconstruct the exact native time", func() {
			So(input.ArrivalTime(), ShouldResemble, expected)
		})
	})
}

func BenchmarkSample_MeasureArrival(t *testing.B) {
	sample := NewSample()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	iteration := 0

	t.ReportAllocs()

	for t.Loop() {
		side := "buy"

		if iteration%2 == 1 {
			side = "sell"
		}

		_, _, _ = sample.MeasureArrival(tradeInput(
			"ALT/EUR",
			side,
			base.Add(time.Duration(iteration)*time.Millisecond),
		))
		iteration++
	}
}

func tradeInput(symbol string, side string, at time.Time) TradeInput {
	return TradeInput{
		Symbol:    symbol,
		Side:      side,
		Timestamp: at,
	}
}
