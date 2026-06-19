package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestBackdoor_Read(testingTB *testing.T) {
	Convey("Given a linear causal table", testingTB, func() {
		stage := NewBackdoor()
		artifact := tableArtifact(16, 1.0, 0.8)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should estimate a finite backdoor effect", func() {
			So(datura.Peek[float64](artifact, "output", "effect"), ShouldNotEqual, 0)
		})
	})
}

func TestLadder_Read_KernelBackdoor(testingTB *testing.T) {
	Convey("Given enough history rows", testingTB, func() {
		stage := NewLadder()
		artifact := tableArtifact(16, 1.0, 0.8)
		artifact.Poke(0.35, "config", "kernelBandwidth")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should return a finite kernel backdoor effect", func() {
			So(datura.Peek[float64](artifact, "output", "intervention"), ShouldBeGreaterThan, 0)
		})
	})
}
