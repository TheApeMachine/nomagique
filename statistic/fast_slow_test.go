package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func fastSlowConfig() *datura.Artifact {
	return datura.Acquire("fast-slow-config", datura.APPJSON).
		Poke("sample", "input").
		Poke("value", "outputKey").
		Poke(float64(3), "config", "fastWindow")
}

func TestFastSlowRead(t *testing.T) {
	Convey("Given a FastSlow stage", t, func() {
		ratio := NewFastSlow(fastSlowConfig())
		artifact := datura.Acquire("test", datura.APPJSON)
		var got float64

		for _, sample := range []float64{1, 1, 1, 10, 10, 10} {
			err := transport.NewFlipFlop(ScalarWire(artifact, "sample", sample), ratio)

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

func TestFastSlowInvertedRead(t *testing.T) {
	Convey("Given an inverted FastSlow stage", t, func() {
		invertedConfig := fastSlowConfig().Poke(1.0, "config", "invert")
		ratio := NewFastSlow(invertedConfig)
		artifact := datura.Acquire("test", datura.APPJSON)
		var got float64

		for _, sample := range []float64{1, 1, 1, 10, 10, 10} {
			err := transport.NewFlipFlop(ScalarWire(artifact, "sample", sample), ratio)

			if err != nil {
				continue
			}

			got = datura.Peek[float64](artifact, "output", "value")
		}

		Convey("It should invert the breakout ratio", func() {
			So(got, ShouldBeLessThan, 1)
		})
	})
}

func BenchmarkFastSlowRead(testingTB *testing.B) {
	fastSlow := NewFastSlow(fastSlowConfig())
	artifact := datura.Acquire("fast-slow-bench", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(ScalarWire(artifact, "sample", 2.0), fastSlow)
	}
}
