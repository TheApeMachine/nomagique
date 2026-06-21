package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestFastSlowSeries(t *testing.T) {
	Convey("Given a FastSlow stage", t, func() {
		ratio := NewFastSlow(3, 1e-6)
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{0, 0, 0, 10, 10, 10} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, ratio)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should compare fast and slow windows", func() {
			So(got, ShouldBeGreaterThan, 1)
		})
	})
}

func TestFastSlowRate(t *testing.T) {
	Convey("Given helper rates", t, func() {
		stream := []float64{0, 0, 0, 10, 10, 10}
		ratio := NewFastSlow(3, 1e-6)
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range stream {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, ratio)

			So(err, ShouldBeNil)
		}

		stageOutput := datura.Peek[float64](artifact, "output", "value")

		Convey("It should match stage output", func() {
			So(FastSlowRate(stream, 3, 1e-6), ShouldEqual, stageOutput)
		})
	})
}
