package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestObservationRingObserve(t *testing.T) {
	Convey("Given an observation ring", t, func() {
		ring := NewObservationRing()
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{0.7, 0.8, 0.9, 0.95} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, ring)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should retain the latest sample", func() {
			So(got, ShouldEqual, 0.95)
		})
	})

	Convey("Given non-positive observations", t, func() {
		ring := NewObservationRing()
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, value := range []float64{0, -1} {
			artifact.Poke(value, "sample")
			err := transport.NewFlipFlop(artifact, ring)

			So(err, ShouldBeNil)
		}

		Convey("It should ignore invalid samples", func() {
			So(len(datura.Peek[[]float64](artifact, "history")), ShouldEqual, 0)
		})
	})
}
