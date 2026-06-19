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
		regime := NewRegime()
		hysteresis := adaptive.NewHysteresis()
		ladder := NewLadder()

		artifact := tableArtifact(16, 1.0, 0.8)
		artifact.Poke(0.35, "config", "kernelBandwidth")
		artifact.Poke(0.0, "paired")

		err := transport.NewFlipFlop(artifact, regime)

		So(err, ShouldBeNil)

		err = transport.NewFlipFlop(artifact, hysteresis)

		So(err, ShouldBeNil)

		err = transport.NewFlipFlop(artifact, ladder)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](ladder.Artifact(), "output", "intervention"), ShouldBeGreaterThan, 0)
	})
}

func BenchmarkLadder_Read(testingTB *testing.B) {
	regime := NewRegime()
	hysteresis := adaptive.NewHysteresis()
	ladder := NewLadder()
	artifact := tableArtifact(16, 1.0, 0.8)
	artifact.Poke(0.35, "config", "kernelBandwidth")

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, regime)
		_ = transport.NewFlipFlop(artifact, hysteresis)
		_ = transport.NewFlipFlop(artifact, ladder)
	}
}
