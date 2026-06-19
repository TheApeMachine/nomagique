package causal

import (
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func graphArtifact(
	parents [][]int,
	treatment int,
	target int,
	controls []int,
) *datura.Artifact {
	artifact := datura.Acquire("test", datura.APPJSON).
		Poke(float64(len(parents)), "config", "graphNodeCount").
		Poke(float64(treatment), "config", "treatment").
		Poke(float64(target), "config", "target")

	for node, parentList := range parents {
		parentValues := make([]float64, len(parentList))

		for index, parent := range parentList {
			parentValues[index] = float64(parent)
		}

		artifact.Poke(parentValues, "config", "graphParent", strconv.Itoa(node))
	}

	if len(controls) > 0 {
		controlValues := make([]float64, len(controls))

		for index, control := range controls {
			controlValues[index] = float64(control)
		}

		artifact.Poke(controlValues, "config", "controls")
	}

	return artifact
}

func TestGraph_Read(testingTB *testing.T) {
	Convey("Given a fork confounder Z -> X and Z -> Y", testingTB, func() {
		stage := NewGraph()
		artifact := graphArtifact([][]int{
			nil,
			{0},
			{0},
			nil,
		}, 1, 2, []int{0})
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should admit controls that block the fork", func() {
			So(datura.Peek[float64](artifact, "output", "admissible"), ShouldEqual, 1)
		})
	})

	Convey("Given a fork without adjustment", testingTB, func() {
		stage := NewGraph()
		artifact := graphArtifact([][]int{
			nil,
			{0},
			{0},
			nil,
		}, 1, 2, nil)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should reject an empty adjustment set", func() {
			So(datura.Peek[float64](artifact, "output", "admissible"), ShouldEqual, 0)
		})
	})

	Convey("Given a descendant control on the treatment path", testingTB, func() {
		stage := NewGraph()
		artifact := graphArtifact([][]int{
			nil,
			{0},
			{1},
			nil,
		}, 1, 3, []int{2})
		err := transport.NewFlipFlop(artifact, stage)

		Convey("It should reject controls that descend from treatment", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "admissible"), ShouldEqual, 0)
		})
	})
}

func BenchmarkGraph_Read(testingTB *testing.B) {
	stage := NewGraph()
	artifact := graphArtifact([][]int{
		nil,
		{0},
		{0, 1},
		{2},
	}, 1, 3, []int{0})

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
