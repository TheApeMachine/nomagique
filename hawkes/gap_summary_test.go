package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGapSummary_Reset(testingTB *testing.T) {
	Convey("Given an existing summary and a new marked arrival sequence", testingTB, func() {
		start := time.Unix(100, 0)
		summary := newGapSummary(4)
		summary.reset([]MarkedEvent{
			{At: start},
			{At: start.Add(3 * time.Second)},
			{At: start.Add(4 * time.Second)},
			{At: start.Add(6 * time.Second)},
		})

		Convey("It should retain exact gaps and their sorted statistics", func() {
			median, ok := summary.median()
			lower, upper, err := summary.quartiles()

			So(ok, ShouldBeTrue)
			So(err, ShouldBeNil)
			So(summary.values, ShouldResemble, []float64{3, 1, 2})
			So(median, ShouldEqual, 2.0)
			So(lower, ShouldEqual, 1.0)
			So(upper, ShouldEqual, 2.25)
		})
	})
}
