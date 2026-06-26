package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func contagionConfig() *datura.Artifact {
	return datura.Acquire("contagion-config", datura.APPJSON).
		Poke(float64(3), "target").
		Poke([]float64{0, 3}, "contagionSkip")
}

func TestContagion_Read(testingTB *testing.T) {
	Convey("Given a populated table", testingTB, func() {
		stage := NewContagion(contagionConfig())
		artifact := tableInbound(16, 1.0)
		err := nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "paired"), ShouldBeGreaterThan, 0)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, datura.Peek[float64](artifact, "paired"))
	})
}
