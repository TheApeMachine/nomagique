package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestIntervalCoupling_Observe(testingTB *testing.T) {
	Convey("Given proportional interval histories", testingTB, func() {
		coupling := NewIntervalCoupling(datura.Acquire("interval-coupling-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(0, "config", "side").
			Poke(float64(1_000), "sample").
			Poke(100.0, "paired")
		err := transport.NewFlipFlop(artifact, coupling)

		So(err, ShouldNotBeNil)

		artifact.Poke(0, "config", "side").
			Poke(float64(2_000), "sample").
			Poke(110.0, "paired")
		err = transport.NewFlipFlop(artifact, coupling)

		So(err, ShouldNotBeNil)

		artifact.Poke(1, "config", "side").
			Poke(float64(1_000), "sample").
			Poke(50.0, "paired")
		err = transport.NewFlipFlop(artifact, coupling)

		So(err, ShouldNotBeNil)

		artifact.Poke(1, "config", "side").
			Poke(float64(2_000), "sample").
			Poke(55.0, "paired")
		err = transport.NewFlipFlop(artifact, coupling)

		So(err, ShouldBeNil)

		value := datura.Peek[float64](artifact, "output", "value")

		Convey("It should estimate unit correlation", func() {
			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCoupling_Observe(testingTB *testing.B) {
	coupling := NewIntervalCoupling(datura.Acquire("interval-coupling-config", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	for step := range 64 {
		epoch := float64((step + 1) * 1_000)
		artifact.Poke(0, "config", "side").Poke(epoch, "sample").Poke(100+float64(step)*0.1, "paired")
		_ = transport.NewFlipFlop(artifact, coupling)
		artifact.Poke(1, "config", "side").Poke(epoch, "sample").Poke(50+float64(step)*0.05, "paired")
		_ = transport.NewFlipFlop(artifact, coupling)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, coupling)
	}
}
