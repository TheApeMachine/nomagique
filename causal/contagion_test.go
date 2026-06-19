package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestContagion_Read(testingTB *testing.T) {
	Convey("Given a populated table", testingTB, func() {
		stage := NewContagion()
		artifact := tableArtifact(16, 1.0, 0.8)
		artifact.Poke([]float64{0, 3}, "config", "contagionSkip")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "paired"), ShouldBeGreaterThan, 0)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, datura.Peek[float64](artifact, "paired"))
	})
}
