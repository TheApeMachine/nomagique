package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestHysteresis_Read(testingTB *testing.T) {
	Convey("Given a hysteresis stage", testingTB, func() {
		stage := NewHysteresis(datura.Acquire("hysteresis-config", datura.APPJSON))

		Convey("It should require consecutive high samples before switching on", func() {
			for range 2 {
				artifact := datura.Acquire("test", datura.APPJSON).
					Poke(1.0, "sample").
					Poke(float64(3), "config", "window")
				err := transport.NewFlipFlop(artifact, stage)

				So(err, ShouldBeNil)
			}

			So(datura.Peek[float64](stage.artifact, "output", "value"), ShouldEqual, 0)

			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(1.0, "sample").
				Poke(float64(3), "config", "window")
			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](stage.artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}

func BenchmarkHysteresis_Read(b *testing.B) {
	stage := NewHysteresis(datura.Acquire("hysteresis-config", datura.APPJSON))

	b.ReportAllocs()

	for b.Loop() {
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(1.0, "sample").
			Poke(float64(2), "config", "window")
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
