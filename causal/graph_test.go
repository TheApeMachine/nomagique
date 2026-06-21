package causal

import (
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func graphConfig(
	parents [][]int,
	treatment int,
	target int,
	controls []int,
) *datura.Artifact {
	config := datura.Acquire("graph-config", datura.APPJSON).
		Poke(float64(len(parents)), "config", "graphNodeCount").
		Poke(float64(treatment), "config", "treatment").
		Poke(float64(target), "config", "target")

	for node, parentList := range parents {
		parentValues := make([]float64, len(parentList))

		for index, parent := range parentList {
			parentValues[index] = float64(parent)
		}

		config.Poke(parentValues, "config", "graphParent", strconv.Itoa(node))
	}

	if len(controls) > 0 {
		controlValues := make([]float64, len(controls))

		for index, control := range controls {
			controlValues[index] = float64(control)
		}

		config.Poke(controlValues, "config", "controls")
	}

	return config
}

func TestGraph_Read(testingTB *testing.T) {
	Convey("Given a fork confounder Z -> X and Z -> Y", testingTB, func() {
		stage := NewGraph(graphConfig([][]int{
			nil,
			{0},
			{0},
			nil,
		}, 1, 2, []int{0}))
		artifact := datura.Acquire("graph-inbound", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should admit controls that block the fork", func() {
			So(datura.Peek[float64](artifact, "output", "admissible"), ShouldEqual, 1)
		})
	})

	Convey("Given a fork without adjustment", testingTB, func() {
		stage := NewGraph(graphConfig([][]int{
			nil,
			{0},
			{0},
			nil,
		}, 1, 2, nil))
		artifact := datura.Acquire("graph-inbound", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should reject an empty adjustment set", func() {
			So(datura.Peek[float64](artifact, "output", "admissible"), ShouldEqual, 0)
		})
	})

	Convey("Given a descendant control on the treatment path", testingTB, func() {
		stage := NewGraph(graphConfig([][]int{
			nil,
			{0},
			{1},
			nil,
		}, 1, 3, []int{2}))
		artifact := datura.Acquire("graph-inbound", datura.APPJSON)

		Convey("It should reject controls that descend from treatment", func() {
			err := transport.NewFlipFlop(artifact, stage)
			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "admissible"), ShouldEqual, 0)
		})
	})
}

func BenchmarkGraph_Read(testingTB *testing.B) {
	stage := NewGraph(graphConfig([][]int{
		nil,
		{0},
		{0, 1},
		{2},
	}, 1, 3, []int{0}))
	artifact := datura.Acquire("graph-inbound", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
