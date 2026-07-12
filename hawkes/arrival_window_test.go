package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestArrivalWindow_Stream(t *testing.T) {
	Convey("Given ordered and out-of-order arrivals inside a bounded window", t, func() {
		start := time.Unix(1000, 0)
		window := NewArrivalWindow(3)
		window.AppendSell(start.Add(time.Second))
		window.AppendBuy(start.Add(4 * time.Second))
		window.AppendSell(start.Add(3 * time.Second))
		window.AppendBuy(start.Add(2 * time.Second))
		stream := window.Stream()

		Convey("It should evict the globally oldest arrival and preserve chronological sides", func() {
			So(stream.BuyTimes(), ShouldResemble, []time.Time{
				start.Add(2 * time.Second),
				start.Add(4 * time.Second),
			})
			So(stream.SellTimes(), ShouldResemble, []time.Time{
				start.Add(3 * time.Second),
			})
			So(stream.Gaps(), ShouldResemble, []float64{1.0, 1.0})
		})
	})
}

func TestArrivalWindow_RetainFrom(t *testing.T) {
	Convey("Given a marked stream and a derived time horizon", t, func() {
		start := time.Unix(1000, 0)
		window := NewArrivalWindow(0)
		window.AppendBuy(start)
		window.AppendSell(start.Add(time.Second))
		window.AppendBuy(start.Add(2 * time.Second))

		window.RetainFrom(start.Add(time.Second))
		stream := window.Stream()

		Convey("It should retain both sides at and after the horizon", func() {
			So(stream.BuyTimes(), ShouldResemble, []time.Time{
				start.Add(2 * time.Second),
			})
			So(stream.SellTimes(), ShouldResemble, []time.Time{
				start.Add(time.Second),
			})
		})
	})
}

func BenchmarkArrivalWindow_Stream(t *testing.B) {
	window := NewArrivalWindow(64)
	start := time.Unix(1000, 0)

	for index := range 128 {
		arrival := start.Add(time.Duration(index) * time.Millisecond)

		if index%2 == 0 {
			window.AppendBuy(arrival)
			continue
		}

		window.AppendSell(arrival)
	}

	t.ReportAllocs()

	for t.Loop() {
		_ = window.Stream()
	}
}
