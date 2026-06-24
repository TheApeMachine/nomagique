package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestWindowSetObserve(testingTB *testing.T) {
	Convey("Given a window set", testingTB, func() {
		windowSet := NewWindowSet(IntervalWireConfig("window-set-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		for index := range 13 {
			artifact = EpochLevelWire(
				artifact,
				float64((index+1)*1_000),
				100+float64(index),
			)
			err := transport.NewFlipFlop(artifact, windowSet)

			if index == 0 {
				So(err, ShouldNotBeNil)
				continue
			}

			So(err, ShouldBeNil)
		}

		Convey("It should publish the latest return magnitude", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}
