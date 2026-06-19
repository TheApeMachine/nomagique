package algorithm_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/algorithm"
)

func pearlArtifact() *datura.Artifact {
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
		Poke(float64(1), "config", "treatmentInverted").
		Poke([]float64{0}, "config", "controlsInverted").
		Poke(float64(1), "config", "conditionLeft").
		Poke(float64(2), "config", "conditionRight").
		Poke([]float64{0, 3}, "config", "contagionSkip").
		Poke(0.35, "config", "kernelBandwidth")
}

func nodeRingFromStreams(streams [][]float64) *algorithm.NodeRing {
	nodeRing := algorithm.NewNodeRing(len(streams), len(streams[0]))

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

func TestPearl_Read(testingTB *testing.T) {
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
		pearl := algorithm.NewPearl()
		pearl.SetNodes(nodes)

		artifact := pearlArtifact()
		err := transport.NewFlipFlop(artifact, pearl)

		So(err, ShouldBeNil)
		So(datura.Peek[int](artifact, "classifier.category"), ShouldBeGreaterThan, 0)
		So(datura.Peek[float64](artifact, "classifier.confidence"), ShouldBeGreaterThan, 0)
	})
}

func BenchmarkPearl_Read(testingTB *testing.B) {
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
	pearl := algorithm.NewPearl()
	pearl.SetNodes(nodes)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact := pearlArtifact()
		_ = transport.NewFlipFlop(artifact, pearl)
	}
}
