package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestRank(testingTB *testing.T) {
	Convey("Given Rank constructor", testingTB, func() {
		empirical := NewRank(datura.Acquire("rank-config", datura.APPJSON))

		Convey("It should return a usable dynamic", func() {
			So(empirical, ShouldNotBeNil)
		})
	})
}

func TestEmpiricalRank_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		empirical := NewRank(datura.Acquire("rank-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})

	Convey("Given empirical rank history", testingTB, func() {
		empirical := NewRank(datura.Acquire("rank-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(10, "sample")
		err := transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		artifact.Poke(5, "sample")
		err = transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return a probability in the unit interval", func() {
			So(got, ShouldBeGreaterThan, 0)
			So(got, ShouldBeLessThan, 1)
		})
	})

	Convey("Given a combined scalar sample", testingTB, func() {
		empirical := NewRank(datura.Acquire("rank-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(10, "sample")
		err := transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		artifact.Poke(8, "sample")
		err = transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		withWork := datura.Peek[float64](artifact, "output", "value")

		combined := NewRank(datura.Acquire("rank-config-combined", datura.APPJSON))
		reference := datura.Acquire("test", datura.APPJSON)

		reference.Poke(10, "sample")
		err = transport.NewFlipFlop(reference, combined)

		So(err, ShouldBeNil)

		reference.Poke(8, "sample")
		err = transport.NewFlipFlop(reference, combined)

		So(err, ShouldBeNil)

		direct := datura.Peek[float64](reference, "output", "value")

		Convey("It should match a single combined scalar", func() {
			So(withWork, ShouldEqual, direct)
		})
	})
}

func TestEmpiricalRank_Reset(testingTB *testing.T) {
	Convey("Given an observed rank", testingTB, func() {
		empirical := NewRank(datura.Acquire("rank-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample")

		err := transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err = transport.NewFlipFlop(resetArtifact, empirical)

		So(err, ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](resetArtifact, "output", "ready"), ShouldEqual, 0)
			So(datura.Peek[float64](resetArtifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkRank_Observe(testingTB *testing.B) {
	empirical := NewRank(datura.Acquire("rank-config-bench", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact.Poke(10, "sample")
	_ = transport.NewFlipFlop(artifact, empirical)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke(10.5, "sample")
		_ = transport.NewFlipFlop(artifact, empirical)
	}
}
