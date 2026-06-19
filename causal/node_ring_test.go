package causal_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/causal"
)

func observeNodeRow(nodeRing *causal.NodeRing, values ...float64) {
	artifact := datura.Acquire("test", datura.APPJSON).Poke(values, "batch")
	err := transport.NewFlipFlop(artifact, nodeRing)

	So(err, ShouldBeNil)
}

func TestNewNodeRing(testingTB *testing.T) {
	Convey("Given NewNodeRing", testingTB, func() {
		nodeRing := causal.NewNodeRing(4, 8)

		Convey("It should start empty", func() {
			So(nodeRing, ShouldNotBeNil)
			So(nodeRing.AlignedLength(), ShouldEqual, 0)
		})
	})
}

func TestNodeRing_Read(testingTB *testing.T) {
	Convey("Given aligned node observations", testingTB, func() {
		nodeRing := causal.NewNodeRing(4, 8)

		for index := range 16 {
			observeNodeRow(
				nodeRing,
				float64(index)*0.1,
				float64(index)*0.2,
				float64(index)*0.5,
				float64(index)*0.05,
			)
		}

		Convey("It should retain bounded aligned history", func() {
			So(nodeRing.AlignedLength(), ShouldEqual, 8)
			So(len(nodeRing.Streams()[3]), ShouldEqual, 8)
		})
	})

	Convey("Given partial node inputs", testingTB, func() {
		nodeRing := causal.NewNodeRing(4, 8)
		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{1}, "batch")
		err := transport.NewFlipFlop(artifact, nodeRing)

		Convey("It should ignore misaligned rows", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
			So(nodeRing.AlignedLength(), ShouldEqual, 0)
		})
	})
}

func BenchmarkNodeRing_Read(b *testing.B) {
	nodeRing := causal.NewNodeRing(4, 16)

	b.ReportAllocs()

	for b.Loop() {
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{0.1, 0.2, 0.5, 0.05}, "batch")
		_ = transport.NewFlipFlop(artifact, nodeRing)
	}
}
