package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewArrivalStream(testingTB *testing.T) {
	Convey("Given unsorted buy and sell times", testingTB, func() {
		start := time.Unix(100, 0)
		stream := NewArrivalStream(
			[]time.Time{
				start.Add(3 * time.Second),
				start,
				start.Add(2 * time.Second),
			},
			[]time.Time{
				start.Add(2500 * time.Millisecond),
				start.Add(500 * time.Millisecond),
			},
		)

		Convey("It should sort both sides", func() {
			buyTimes := stream.BuyTimes()

			So(len(buyTimes), ShouldEqual, 3)
			So(buyTimes[0], ShouldEqual, start)
			So(buyTimes[1], ShouldEqual, start.Add(2*time.Second))
			So(buyTimes[2], ShouldEqual, start.Add(3*time.Second))

			sellTimes := stream.SellTimes()

			So(len(sellTimes), ShouldEqual, 2)
			So(sellTimes[0], ShouldEqual, start.Add(500*time.Millisecond))
			So(sellTimes[1], ShouldEqual, start.Add(2500*time.Millisecond))
		})
	})
}

func TestArrivalStream_Marked(testingTB *testing.T) {
	Convey("Given interleaved buy and sell arrivals", testingTB, func() {
		start := time.Unix(200, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(2 * time.Second), start.Add(4 * time.Second)},
			[]time.Time{start.Add(time.Second), start.Add(3 * time.Second)},
		)
		marked := stream.Marked()

		Convey("It should merge in chronological order", func() {
			So(len(marked), ShouldEqual, 5)
			So(marked[0].Side, ShouldEqual, sideBuy)
			So(marked[1].Side, ShouldEqual, sideSell)
			So(marked[2].Side, ShouldEqual, sideBuy)
			So(marked[3].Side, ShouldEqual, sideSell)
			So(marked[4].Side, ShouldEqual, sideBuy)
		})
	})

	Convey("Given equal timestamps on both sides", testingTB, func() {
		start := time.Unix(300, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(time.Second)},
			[]time.Time{start, start.Add(time.Second)},
		)
		marked := stream.Marked()

		Convey("It should prefer buy on ties", func() {
			So(len(marked), ShouldEqual, 4)
			So(marked[0].Side, ShouldEqual, sideBuy)
			So(marked[1].Side, ShouldEqual, sideSell)
			So(marked[2].Side, ShouldEqual, sideBuy)
			So(marked[3].Side, ShouldEqual, sideSell)
		})
	})

	Convey("Given adversarial empty streams", testingTB, func() {
		stream := NewArrivalStream(nil, nil)

		Convey("It should return no marked events", func() {
			So(stream.Marked(), ShouldBeEmpty)
		})
	})

	Convey("Given buy-only arrivals", testingTB, func() {
		start := time.Unix(400, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(time.Second)},
			nil,
		)
		marked := stream.Marked()

		Convey("It should mark every event as buy", func() {
			So(len(marked), ShouldEqual, 2)

			for _, event := range marked {
				So(event.Side, ShouldEqual, sideBuy)
			}
		})
	})

	Convey("Given sell-only arrivals", testingTB, func() {
		start := time.Unix(500, 0)
		stream := NewArrivalStream(
			nil,
			[]time.Time{start, start.Add(time.Second)},
		)
		marked := stream.Marked()

		Convey("It should mark every event as sell", func() {
			So(len(marked), ShouldEqual, 2)

			for _, event := range marked {
				So(event.Side, ShouldEqual, sideSell)
			}
		})
	})

	Convey("Given a caller changes one returned marked event", testingTB, func() {
		start := time.Unix(600, 0)
		stream := NewArrivalStream(
			[]time.Time{start},
			[]time.Time{start.Add(time.Second)},
		)
		marked := stream.Marked()
		marked[0].At = start.Add(time.Hour)

		Convey("It should keep the stream's merged arrivals immutable", func() {
			So(stream.Marked()[0].At, ShouldEqual, start)
		})
	})
}

func TestArrivalStream_Gaps(testingTB *testing.T) {
	Convey("Given marked arrivals with a simultaneous buy and sell", testingTB, func() {
		start := time.Unix(700, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(2 * time.Second)},
			[]time.Time{start, start.Add(time.Second)},
		)

		Convey("It should return only the positive inter-arrival gaps", func() {
			So(stream.Gaps(), ShouldResemble, []float64{1, 1})
		})
	})
}

func TestArrivalStream_Span(testingTB *testing.T) {
	Convey("Given a marked stream and measurement horizon", testingTB, func() {
		start := time.Unix(800, 0)
		stream := NewArrivalStream(
			[]time.Time{start.Add(time.Second)},
			[]time.Time{start},
		)

		Convey("It should measure from the first merged arrival", func() {
			So(stream.Span(start.Add(3*time.Second)), ShouldEqual, 3.0)
		})

		Convey("It should reject a horizon before the first arrival", func() {
			So(stream.Span(start.Add(-time.Second)), ShouldEqual, 0.0)
		})
	})
}
