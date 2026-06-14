package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGraph_BackdoorAdmissible(testingTB *testing.T) {
	Convey("Given a fork confounder Z -> X and Z -> Y", testingTB, func() {
		graph, err := NewGraph([][]int{
			nil,
			{0},
			{0},
			nil,
		})

		So(err, ShouldBeNil)

		Convey("It should admit controls that block the fork", func() {
			admissible, admErr := graph.BackdoorAdmissible(1, 2, []int{0})

			So(admErr, ShouldBeNil)
			So(admissible, ShouldBeTrue)
		})

		Convey("It should reject an empty adjustment set", func() {
			admissible, admErr := graph.BackdoorAdmissible(1, 2, nil)

			So(admErr, ShouldBeNil)
			So(admissible, ShouldBeFalse)
		})
	})

	Convey("Given a descendant control on the treatment path", testingTB, func() {
		graph, err := NewGraph([][]int{
			nil,
			{0},
			{1},
			nil,
		})

		So(err, ShouldBeNil)

		admissible, admErr := graph.BackdoorAdmissible(1, 3, []int{2})

		Convey("It should reject controls that descend from treatment", func() {
			So(admErr, ShouldBeNil)
			So(admissible, ShouldBeFalse)
		})
	})
}

func BenchmarkGraph_BackdoorAdmissible(testingTB *testing.B) {
	graph, err := NewGraph([][]int{
		nil,
		{0},
		{0, 1},
		{2},
	})

	if err != nil {
		testingTB.Fatal(err)
	}

	for testingTB.Loop() {
		_, _ = graph.BackdoorAdmissible(1, 3, []int{0})
	}
}
