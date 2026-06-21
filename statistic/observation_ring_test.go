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

		Convey("It should retain observation history", func() {
			history := datura.Peek[[]float64](artifact, "history")

			So(len(history), ShouldEqual, 4)
			So(history[len(history)-1], ShouldEqual, 0.95)
		})
	})

	Convey("Given a long run of similar samples", t, func() {
		ring := NewObservationRing()
		artifact := datura.Acquire("test", datura.APPJSON)

		for index := range 100 {
			sample := 1.0 + float64(index%3)*0.01
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, ring)

			So(err, ShouldBeNil)
		}

		history := datura.Peek[[]float64](artifact, "history")

		Convey("It should bound retained history", func() {
			So(len(history), ShouldBeLessThan, 20)
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
