package timeline

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTimelineGaps(testingTB *testing.T) {
	Convey("Given duplicate and positive inter-arrival times", testingTB, func() {
		start := time.Unix(0, 0)
		eventTimeline := New([]time.Time{
			start,
			start.Add(2 * time.Second),
			start.Add(2 * time.Second),
			start.Add(5 * time.Second),
		})

		Convey("It should keep only strictly positive gaps", func() {
			gaps := eventTimeline.Gaps()

			So(len(gaps), ShouldEqual, 2)
			So(gaps[0], ShouldEqual, 2)
			So(gaps[1], ShouldEqual, 3)
		})
	})
}

func TestTimelineSpan(testingTB *testing.T) {
	Convey("Given a timeline and a later horizon", testingTB, func() {
		start := time.Unix(0, 0)
		eventTimeline := New([]time.Time{start, start.Add(time.Second)})

		Convey("It should return elapsed seconds from the first event", func() {
			So(eventTimeline.Span(start.Add(4*time.Second)), ShouldEqual, 4)
		})
	})
}

func TestTimelineNewSortsUnsortedInput(testingTB *testing.T) {
	Convey("Given unsorted timestamps", testingTB, func() {
		start := time.Unix(0, 0)
		eventTimeline := New([]time.Time{
			start.Add(3 * time.Second),
			start,
			start.Add(time.Second),
		})

		Convey("It should return a sorted timeline", func() {
			So(eventTimeline.Times()[0].Equal(start), ShouldBeTrue)
		})
	})
}
