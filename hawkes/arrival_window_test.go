package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestArrivalWindow_Stream(testingTB *testing.T) {
	Convey("Given ordered and out-of-order arrivals inside a bounded window", testingTB, func() {
		start := time.Unix(1000, 0)
		window := NewArrivalWindow(2)
		window.AppendBuy(start.Add(3 * time.Second))
		window.AppendBuy(start.Add(time.Second))
		window.AppendBuy(start.Add(2 * time.Second))
		window.AppendSell(start.Add(2500 * time.Millisecond))
		window.AppendSell(start.Add(1500 * time.Millisecond))
		stream := window.Stream()

		Convey("It should retain the latest arrivals per side and expose sorted Hawkes input", func() {
			So(stream.BuyTimes(), ShouldResemble, []time.Time{
				start.Add(time.Second),
				start.Add(2 * time.Second),
			})
			So(stream.SellTimes(), ShouldResemble, []time.Time{
				start.Add(1500 * time.Millisecond),
				start.Add(2500 * time.Millisecond),
			})
			So(stream.Gaps(), ShouldResemble, []float64{0.5, 0.5, 0.5})
		})
	})
}

func BenchmarkArrivalWindow_Stream(testingTB *testing.B) {
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

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = window.Stream()
	}
}
