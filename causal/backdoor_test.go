package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func backdoorConfig() *datura.Artifact {
	return datura.Acquire("backdoor-config", datura.APPJSON).
		Poke(float64(3), "config", "target").
		Poke(float64(1), "config", "treatment").
		Poke([]float64{0}, "config", "controls").
		Poke(float64(12), "config", "minHistory")
}

func TestBackdoor_Read(testingTB *testing.T) {
	Convey("Given config on the constructor artifact and table rows on inbound wire", testingTB, func() {
		stage := NewBackdoor(backdoorConfig())
		artifact := tableInbound(16, 1.0)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should estimate a finite backdoor effect", func() {
			So(datura.Peek[float64](artifact, "output", "effect"), ShouldNotEqual, 0)
		})
	})
}

func TestLadder_Read_KernelBackdoor(testingTB *testing.T) {
	Convey("Given enough history rows", testingTB, func() {
		config := causalPipelineConfig(0.8)
		config.Poke(0.35, "config", "kernelBandwidth")
		stage := NewLadder(config)
		artifact := tableInbound(16, 1.0)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should return a finite kernel backdoor effect", func() {
			So(datura.Peek[float64](artifact, "output", "intervention"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkBackdoor_Read(testingTB *testing.B) {
	stage := NewBackdoor(backdoorConfig())
	artifact := tableInbound(16, 1.0)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
