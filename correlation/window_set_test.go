package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWindowSetSnapshot(testingTB *testing.T) {
	Convey("Given a window set", testingTB, func() {
		windowSet := NewWindowSet(16)

		for index := range 13 {
			observeEpochLevel(windowSet, int64((index+1)*1_000), 100+float64(index))
		}

		snapshot := windowSet.Snapshot(TierWindows{
			Fast:   4,
			Medium: 8,
			Slow:   12,
		})

		Convey("It should materialize tier views", func() {
			So(snapshot.Fast.Len(), ShouldEqual, 4)
			So(snapshot.Medium.Len(), ShouldEqual, 8)
			So(snapshot.Slow.Len(), ShouldEqual, 12)
		})
	})
}
