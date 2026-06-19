package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/equation"
)

func pearlArtifact(contagionBreak float64) *datura.Artifact {
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

	return datura.Acquire("test", datura.APPJSON).
		Poke(float64(3), "config", "target").
		Poke(float64(12), "config", "minHistory").
		Poke(float64(2), "config", "treatmentNormal").
		Poke([]float64{0, 1}, "config", "controlsNormal").
		Poke([]float64{0, 3}, "config", "contagionSkip").
		Poke(0.35, "config", "kernelBandwidth").
		Poke(contagionBreak, "config", "contagionBreak").
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
}

func nodeRingFromStreams(streams [][]float64) *causal.NodeRing {
	nodeRing := causal.NewNodeRing(len(streams), len(streams[0]))

	for rowIndex := range streams[0] {
		row := make([]float64, len(streams))

		for nodeIndex := range streams {
			row[nodeIndex] = streams[nodeIndex][rowIndex]
		}

		artifact := datura.Acquire("test", datura.APPJSON).Poke(row, "batch")
		_ = transport.NewFlipFlop(artifact, nodeRing)
	}

	return nodeRing
}

func TestRegimeLadder_Read(testingTB *testing.T) {
	Convey("Given aligned node streams with causal structure", testingTB, func() {
		nodeZero := make([]float64, 16)
		nodeOne := make([]float64, 16)
		nodeTwo := make([]float64, 16)
		nodeThree := make([]float64, 16)

		for index := range nodeZero {
			nodeZero[index] = float64(index) * 0.1
			nodeOne[index] = float64(index) * 0.2
			nodeTwo[index] = float64(index) * 0.5
			nodeThree[index] = float64(index) * 0.05
		}

		streams := [][]float64{nodeZero, nodeOne, nodeTwo, nodeThree}
		nodes := nodeRingFromStreams(streams)
		ladderStage := causal.NewLadder()
		regimeLadder := equation.NewRegimeLadder(ladderStage)
		artifact := pearlArtifact(0.8).Poke(0.0, "paired")

		err := transport.NewFlipFlop(artifact, nodes)
		So(err, ShouldBeNil)

		err = transport.NewFlipFlop(artifact, regimeLadder)
		So(err, ShouldBeNil)
		So(datura.Peek[float64](ladderStage.Artifact(), "output", "intervention"), ShouldBeGreaterThan, 0)
	})
}

func TestReading_Read(testingTB *testing.T) {
	Convey("Given a ladder reading score source", testingTB, func() {
		ladderStage := causal.NewLadder()
		reading := equation.NewReading(ladderStage, "uplift")

		So(reading, ShouldNotBeNil)
	})
}

func BenchmarkRegimeLadder_Read(testingTB *testing.B) {
	nodeZero := make([]float64, 16)
	nodeOne := make([]float64, 16)
	nodeTwo := make([]float64, 16)
	nodeThree := make([]float64, 16)

	for index := range nodeZero {
		nodeZero[index] = float64(index) * 0.1
		nodeOne[index] = float64(index) * 0.2
		nodeTwo[index] = float64(index) * 0.5
		nodeThree[index] = float64(index) * 0.05
	}

	streams := [][]float64{nodeZero, nodeOne, nodeTwo, nodeThree}
	nodes := nodeRingFromStreams(streams)
	ladderStage := causal.NewLadder()
	regimeLadder := equation.NewRegimeLadder(ladderStage)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact := pearlArtifact(0.8).Poke(0.0, "paired")
		_ = transport.NewFlipFlop(artifact, nodes)
		_ = transport.NewFlipFlop(artifact, regimeLadder)
	}
}
