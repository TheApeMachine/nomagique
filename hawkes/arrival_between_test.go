package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestArrivalStream_Between(testingTB *testing.T) {
	Convey("Given a sorted two-sided stream and inclusive interval", testingTB, func() {
		start := time.Unix(2000, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(2 * time.Second), start.Add(4 * time.Second)},
			[]time.Time{start.Add(time.Second), start.Add(3 * time.Second)},
		)
		clipped := stream.Between(
			start.Add(time.Second),
			start.Add(3*time.Second),
		)

		Convey("It should retain both interval boundaries without changing order", func() {
			So(clipped.BuyTimes(), ShouldResemble, []time.Time{start.Add(2 * time.Second)})
			So(clipped.SellTimes(), ShouldResemble, []time.Time{
				start.Add(time.Second),
				start.Add(3 * time.Second),
			})
		})
	})
}

func TestArrivalStream_BetweenInto(testingTB *testing.T) {
	Convey("Given a stream interval and reusable workspace", testingTB, func() {
		start := time.Unix(2500, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(2 * time.Second)},
			[]time.Time{start.Add(time.Second), start.Add(3 * time.Second)},
		)
		workspace := NewArrivalWorkspace()
		clipped := stream.BetweenInto(
			start.Add(time.Second),
			start.Add(2*time.Second),
			workspace,
		)

		Convey("It should preserve the same inclusive clipping behavior", func() {
			So(clipped.BuyTimes(), ShouldResemble, []time.Time{start.Add(2 * time.Second)})
			So(clipped.SellTimes(), ShouldResemble, []time.Time{start.Add(time.Second)})
		})
	})
}
