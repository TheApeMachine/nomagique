package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/equation"
)

func regimeLadderConfig(contagionBreak float64) *datura.Artifact {
	return datura.Acquire("regime-ladder-config", datura.APPJSON).
		Poke(float64(3), "config", "target").
		Poke(float64(12), "config", "minHistory").
		Poke(float64(2), "config", "treatmentNormal").
		Poke([]float64{0, 1}, "config", "controlsNormal").
		Poke([]float64{0, 3}, "config", "contagionSkip").
		Poke(0.35, "config", "kernelBandwidth").
		Poke(contagionBreak, "config", "contagionBreak")
}

func regimeLadderInbound() *datura.Artifact {
	nodeCount := 4
	rowCount := 16
	flat := make([]float64, 0, rowCount*nodeCount)

	for rowIndex := range rowCount {
		flat = append(flat,
			float64(rowIndex)*0.1,
			float64(rowIndex)*0.2,
			float64(rowIndex)*0.5,
			float64(rowIndex)*0.05,
		)
	}

	return datura.Acquire("regime-ladder-inbound", datura.APPJSON).
		Poke(0.0, "paired").
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
}

func TestRegimeLadder_Read(testingTB *testing.T) {
	Convey("Given aligned node streams with causal structure", testingTB, func() {
		regimeLadder := equation.NewRegimeLadder(regimeLadderConfig(0.8))
		artifact := regimeLadderInbound()
		err := transport.NewFlipFlop(artifact, regimeLadder)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "intervention"), ShouldBeGreaterThan, 0)
	})
}

func TestReading_Read(testingTB *testing.T) {
	Convey("Given a ladder reading score source", testingTB, func() {
		reading := equation.NewReading("uplift")

		So(reading, ShouldNotBeNil)
	})
}

func BenchmarkRegimeLadder_Read(testingTB *testing.B) {
	regimeLadder := equation.NewRegimeLadder(regimeLadderConfig(0.8))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact := regimeLadderInbound()
		_ = transport.NewFlipFlop(artifact, regimeLadder)
	}
}
