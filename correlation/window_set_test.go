package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestWindowSetSnapshot(testingTB *testing.T) {
	Convey("Given a window set", testingTB, func() {
		windowSet := NewWindowSet(16)
		artifact := datura.Acquire("test", datura.APPJSON)

		for index := range 13 {
			artifact.Poke(float64((index+1)*1_000), "sample").
				Poke(100+float64(index), "paired")
			err := transport.NewFlipFlop(artifact, windowSet)

			So(err, ShouldBeNil)
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
