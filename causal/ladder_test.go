package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/adaptive"
)

func TestLadder_Read(testingTB *testing.T) {
	Convey("Given aligned node rows with causal structure", testingTB, func() {
		config := causalPipelineConfig(0.8)
		config.Poke(0.35, "kernelBandwidth")
		config.Poke(float64(12), "history")
		regime := NewRegime(config)
		hysteresis := adaptive.NewHysteresis(config)
		ladder := NewLadder(config)

		artifact := tableInbound(16, 1.0)
		artifact.Poke(0.0, "paired")

		err := transport.NewFlipFlop(artifact, regime)

		So(err, ShouldBeNil)

		err = transport.NewFlipFlop(artifact, hysteresis)

		So(err, ShouldBeNil)

		err = transport.NewFlipFlop(artifact, ladder)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "intervention"), ShouldBeGreaterThan, 0)
	})
}

func BenchmarkLadder_Read(testingTB *testing.B) {
	config := causalPipelineConfig(0.8)
	config.Poke(0.35, "kernelBandwidth")
	config.Poke(float64(12), "history")
	regime := NewRegime(config)
	hysteresis := adaptive.NewHysteresis(config)
	ladder := NewLadder(config)
	artifact := tableInbound(16, 1.0)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, regime)
		_ = transport.NewFlipFlop(artifact, hysteresis)
		_ = transport.NewFlipFlop(artifact, ladder)
	}
}
