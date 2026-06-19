package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestWindowSetObserve(testingTB *testing.T) {
	Convey("Given a window set", testingTB, func() {
		windowSet := NewWindowSet()
		artifact := datura.Acquire("test", datura.APPJSON)

		for index := range 13 {
			artifact.Poke(float64((index+1)*1_000), "sample").
				Poke(100+float64(index), "paired")
			err := transport.NewFlipFlop(artifact, windowSet)

			So(err, ShouldBeNil)
		}

		Convey("It should publish the latest return magnitude", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}
