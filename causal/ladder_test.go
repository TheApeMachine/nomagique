package causal

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
)

func TestLadder_Read(testingTB *testing.T) {
	Convey("Given no inbound frame", testingTB, func() {
		ladder := NewLadder(causalPipelineConfig(0.8))
		buffer := make([]byte, 4096)

		n, err := ladder.Read(buffer)

		Convey("It should report no frame without trying to unpack stale payload", func() {
			So(n, ShouldEqual, 0)
			So(err, ShouldEqual, io.EOF)
		})
	})

	Convey("Given an empty inbound frame", testingTB, func() {
		ladder := NewLadder(causalPipelineConfig(0.8))
		buffer := make([]byte, 4096)

		n, writeErr := ladder.Write(nil)
		readN, readErr := ladder.Read(buffer)

		Convey("It should drain cleanly without validation noise", func() {
			So(n, ShouldEqual, 0)
			So(writeErr, ShouldBeNil)
			So(readN, ShouldEqual, 0)
			So(readErr, ShouldEqual, io.EOF)
		})
	})

	Convey("Given aligned node rows with causal structure", testingTB, func() {
		config := causalPipelineConfig(0.8)
		config.Poke(0.35, "kernelBandwidth")
		config.Poke(float64(12), "minHistory")
		config.Poke(float64(12), "history")
		regime := NewRegime(config)
		hysteresis := adaptive.NewHysteresis(config)
		ladder := NewLadder(config)

		artifact := tableInbound(16, 1.0)
		artifact.Poke(0.0, "paired")

		err := nomagique.RoundTripArtifact(artifact, regime)

		So(err, ShouldBeNil)

		err = nomagique.RoundTripArtifact(artifact, hysteresis)

		So(err, ShouldBeNil)

		err = nomagique.RoundTripArtifact(artifact, ladder)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "intervention"), ShouldBeGreaterThan, 0)
	})
}

func TestLadder_ReadWarmup(testingTB *testing.T) {
	Convey("Given fewer table rows than minHistory", testingTB, func() {
		config := causalPipelineConfig(0.8)
		config.Poke(float64(12), "minHistory")
		ladder := NewLadder(config)
		artifact := tableInbound(4, 1.0)

		err := nomagique.RoundTripArtifact(artifact, ladder)

		So(err, ShouldNotBeNil)
	})
}

func TestLadder_ReadKernelMiss(testingTB *testing.T) {
	Convey("Given a kernel adjustment that cannot be fitted", testingTB, func() {
		config := causalPipelineConfig(0.8)
		config.Poke(0.35, "kernelBandwidth")
		config.Poke(float64(12), "minHistory")
		config.Poke([]float64{99}, "controlsNormal")
		ladder := NewLadder(config)
		artifact := tableInbound(16, 1.0)

		err := nomagique.RoundTripArtifact(artifact, ladder)

		So(err, ShouldNotBeNil)
	})
}

func BenchmarkLadder_Read(testingTB *testing.B) {
	config := causalPipelineConfig(0.8)
	config.Poke(0.35, "kernelBandwidth")
	config.Poke(float64(12), "minHistory")
	config.Poke(float64(12), "history")
	regime := NewRegime(config)
	hysteresis := adaptive.NewHysteresis(config)
	ladder := NewLadder(config)
	artifact := tableInbound(16, 1.0)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = nomagique.RoundTripArtifact(artifact, regime)
		_ = nomagique.RoundTripArtifact(artifact, hysteresis)
		_ = nomagique.RoundTripArtifact(artifact, ladder)
	}
}
