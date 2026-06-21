package algorithm_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/algorithm"
)

func pearlConfig() *datura.Artifact {
	return datura.Acquire("pearl-config", datura.APPJSON).
		Poke(float64(3), "config", "target").
		Poke(float64(12), "config", "minHistory").
		Poke(float64(2), "config", "treatmentNormal").
		Poke([]float64{0, 1}, "config", "controlsNormal").
		Poke(float64(1), "config", "treatmentInverted").
		Poke([]float64{0}, "config", "controlsInverted").
		Poke(float64(1), "config", "conditionLeft").
		Poke(float64(2), "config", "conditionRight").
		Poke([]float64{0, 3}, "config", "contagionSkip").
		Poke(0.35, "config", "kernelBandwidth").
		Poke(0.8, "config", "contagionBreak")
}

func pearlInbound() *datura.Artifact {
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

	return datura.Acquire("pearl-inbound", datura.APPJSON).
		Poke(0.0, "paired").
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
}

func TestPearl_Read(testingTB *testing.T) {
	Convey("Given aligned node streams with causal structure", testingTB, func() {
		pearl := algorithm.NewPearl(pearlConfig())
		artifact := pearlInbound()
		err := transport.NewFlipFlop(artifact, pearl)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "intervention"), ShouldBeGreaterThan, 0)
	})
}

func BenchmarkPearl_Read(testingTB *testing.B) {
	pearl := algorithm.NewPearl(pearlConfig())

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact := pearlInbound()
		_ = transport.NewFlipFlop(artifact, pearl)
	}
}
