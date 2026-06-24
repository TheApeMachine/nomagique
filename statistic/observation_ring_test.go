package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func observationRingConfig(name string, capacity float64) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "input").
		Poke("value", "outputKey").
		Poke(capacity, "config", "capacity")
}

func TestObservationRingRead(t *testing.T) {
	Convey("Given an observation ring", t, func() {
		ring := NewObservationRing(observationRingConfig("observation-ring-config", 8))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{0.7, 0.8, 0.9, 0.95} {
			err := transport.NewFlipFlop(ScalarWire(artifact, "sample", sample), ring)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should retain the latest sample", func() {
			So(got, ShouldEqual, 0.95)
		})

		Convey("It should retain observation history", func() {
			history := datura.Peek[[]float64](ring.artifact, "history")

			So(len(history), ShouldEqual, 4)
			So(history[len(history)-1], ShouldEqual, 0.95)
		})
	})

	Convey("Given a long run of similar samples", t, func() {
		ring := NewObservationRing(observationRingConfig("observation-ring-config-long", 16))
		artifact := datura.Acquire("test", datura.APPJSON)

		for index := range 100 {
			sample := 1.0 + float64(index%3)*0.01
			err := transport.NewFlipFlop(ScalarWire(artifact, "sample", sample), ring)

			So(err, ShouldBeNil)
		}

		history := datura.Peek[[]float64](ring.artifact, "history")

		Convey("It should bound retained history", func() {
			So(len(history), ShouldBeLessThan, 20)
		})
	})

	Convey("Given non-positive observations", t, func() {
		ring := NewObservationRing(observationRingConfig("observation-ring-config-invalid", 8))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, value := range []float64{0, -1} {
			err := transport.NewFlipFlop(ScalarWire(artifact, "sample", value), ring)

			So(err, ShouldNotBeNil)
		}

		Convey("It should reject invalid samples", func() {
			So(len(datura.Peek[[]float64](ring.artifact, "history")), ShouldEqual, 0)
		})
	})
}
