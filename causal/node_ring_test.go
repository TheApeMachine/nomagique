package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestNodeRingRead(t *testing.T) {
	Convey("Given a NodeRing stage", t, func() {
		config := datura.Acquire("node-ring-config", datura.APPJSON).
			Poke(2.0, "nodeCount").
			Poke(8.0, "capacity")
		ring := NewNodeRing(config)

		Convey("It should accumulate aligned rows", func() {
			artifact := datura.Acquire("node-ring-test", datura.APPJSON).
				Poke([]float64{1.0, 2.0}, "batch")

			err := transport.NewFlipFlop(artifact, ring)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 2.0)
			So(datura.Peek[string](artifact, "root"), ShouldEqual, "output")
			So(datura.Peek[[]string](artifact, "inputs"), ShouldContain, "value")
		})
	})
}

func BenchmarkNodeRingRead(b *testing.B) {
	config := datura.Acquire("node-ring-bench", datura.APPJSON).
		Poke(2.0, "nodeCount").
		Poke(8.0, "capacity")
	ring := NewNodeRing(config)
	artifact := datura.Acquire("node-ring-bench-artifact", datura.APPJSON).
		Poke([]float64{1.0, 2.0}, "batch")

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, ring)
	}
}
