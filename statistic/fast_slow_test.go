package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func fastSlowConfig() *datura.Artifact {
	return datura.Acquire("fast-slow-config", datura.APPJSON).
		Poke(float64(3), "config", "fastWindow").
		Poke(1e-6, "config", "epsilon")
}

func TestFastSlowSeries(t *testing.T) {
	Convey("Given a FastSlow stage", t, func() {
		ratio := NewFastSlow(fastSlowConfig())
		artifact := datura.Acquire("test", datura.APPJSON)
		var got float64

		for _, sample := range []float64{0, 0, 0, 10, 10, 10} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, ratio)

			if err != nil {
				continue
			}

			got = datura.Peek[float64](artifact, "output", "value")
		}

		Convey("It should compare fast and slow windows", func() {
			So(got, ShouldBeGreaterThan, 1)
		})
	})
}

func TestFastSlowRate(t *testing.T) {
	Convey("Given helper rates", t, func() {
		stream := []float64{0, 0, 0, 10, 10, 10}
		ratio := NewFastSlow(fastSlowConfig())
		artifact := datura.Acquire("test", datura.APPJSON)

		var got float64

		for _, sample := range stream {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, ratio)

			if err != nil {
				continue
			}

			got = datura.Peek[float64](artifact, "output", "value")
		}

		stageOutput := got

		Convey("It should match stage output", func() {
			So(FastSlowRate(stream, 3, 1e-6), ShouldEqual, stageOutput)
		})
	})
}
