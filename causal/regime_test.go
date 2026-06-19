package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestRegime_Read(testingTB *testing.T) {
	Convey("Given low contagion and a populated table", testingTB, func() {
		stage := NewRegime()
		artifact := tableArtifact(16, 0.1, 0.8)
		artifact.Poke(0.1, "paired")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](stage.artifact, "output", "rawInverted"), ShouldEqual, 0)
	})

	Convey("Given contagion above break", testingTB, func() {
		stage := NewRegime()
		artifact := tableArtifact(16, 0.1, 0.5)
		artifact.Poke(0.95, "paired")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](stage.artifact, "output", "rawInverted"), ShouldEqual, 1)
	})
}

func tableArtifact(rowCount int, step float64, contagionBreak float64) *datura.Artifact {
	nodeCount := 4
	flat := make([]float64, 0, rowCount*nodeCount)

	for rowIndex := range rowCount {
		flat = append(flat,
			float64(rowIndex)*step*0.1,
			float64(rowIndex)*step*0.2,
			float64(rowIndex)*step*0.5,
			float64(rowIndex)*step*0.05,
		)
	}

	return datura.Acquire("test", datura.APPJSON).
		Poke(float64(3), "config", "target").
		Poke(float64(12), "config", "minHistory").
		Poke(float64(2), "config", "treatmentNormal").
		Poke([]float64{0, 1}, "config", "controlsNormal").
		Poke(float64(1), "config", "treatmentInverted").
		Poke([]float64{0, 2}, "config", "controlsInverted").
		Poke(contagionBreak, "config", "contagionBreak").
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
}
