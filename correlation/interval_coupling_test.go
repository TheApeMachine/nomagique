package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestIntervalCoupling_Observe(testingTB *testing.T) {
	Convey("Given proportional interval histories", testingTB, func() {
		left := NewIntervalSeries(8)
		right := NewIntervalSeries(8)
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(float64(1_000), "sample").Poke(100.0, "paired")
		err := transport.NewFlipFlop(artifact, left)

		So(err, ShouldBeNil)

		artifact.Poke(float64(2_000), "sample").Poke(110.0, "paired")
		err = transport.NewFlipFlop(artifact, left)

		So(err, ShouldBeNil)

		artifact.Poke(float64(1_000), "sample").Poke(50.0, "paired")
		err = transport.NewFlipFlop(artifact, right)

		So(err, ShouldBeNil)

		artifact.Poke(float64(2_000), "sample").Poke(55.0, "paired")
		err = transport.NewFlipFlop(artifact, right)

		So(err, ShouldBeNil)

		coupling := NewIntervalCoupling(left, right)
		trigger := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(trigger, coupling)

		So(err, ShouldBeNil)

		value := datura.Peek[float64](trigger, "output", "value")

		Convey("It should estimate unit correlation", func() {
			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCoupling_Observe(testingTB *testing.B) {
	left := NewIntervalSeries(64)
	right := NewIntervalSeries(64)
	artifact := datura.Acquire("test", datura.APPJSON)

	for step := range 64 {
		epoch := float64((step + 1) * 1_000)
		artifact.Poke(epoch, "sample").Poke(100+float64(step)*0.1, "paired")
		_ = transport.NewFlipFlop(artifact, left)
		artifact.Poke(epoch, "sample").Poke(50+float64(step)*0.05, "paired")
		_ = transport.NewFlipFlop(artifact, right)
	}

	coupling := NewIntervalCoupling(left, right)
	trigger := datura.Acquire("test", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(trigger, coupling)
	}
}
