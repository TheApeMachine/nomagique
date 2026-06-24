package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestNewRank(testingTB *testing.T) {
	Convey("Given Rank constructor", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))

		Convey("It should return a usable dynamic", func() {
			So(empirical, ShouldNotBeNil)
		})
	})
}

func TestRank_Read(testingTB *testing.T) {
	Convey("Given empty inbound wire", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, empirical)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given empirical rank history", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		err := transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		scalarWire(artifact, "sample", 5)
		err = transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return a probability in the unit interval", func() {
			So(got, ShouldBeGreaterThan, 0)
			So(got, ShouldBeLessThan, 1)
		})
	})

	Convey("Given sequential scalar samples", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		err := transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		scalarWire(artifact, "sample", 8)
		err = transport.NewFlipFlop(artifact, empirical)

		So(err, ShouldBeNil)

		withWork := datura.Peek[float64](artifact, "output", "value")

		combined := NewRank(rankConfig("rank-config-combined"))
		reference := datura.Acquire("test", datura.APPJSON)

		scalarWire(reference, "sample", 10)
		err = transport.NewFlipFlop(reference, combined)

		So(err, ShouldBeNil)

		scalarWire(reference, "sample", 8)
		err = transport.NewFlipFlop(reference, combined)

		So(err, ShouldBeNil)

		direct := datura.Peek[float64](reference, "output", "value")

		Convey("It should match a single combined scalar", func() {
			So(withWork, ShouldEqual, direct)
		})
	})

	Convey("Given reset after observation", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)

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

func TestRankState_Observe(testingTB *testing.T) {
	Convey("Given a fresh rank state", testingTB, func() {
		state := RankState{}

		Convey("When bootstrapping", func() {
			rank := state.Observe(5)

			Convey("It should return unit rank", func() {
				So(state.Ready, ShouldBeTrue)
				So(rank, ShouldEqual, 1)
				So(state.Count, ShouldEqual, 1)
			})
		})
	})

	Convey("Given rank history", testingTB, func() {
		state := RankState{}
		_ = state.Observe(10)
		value := state.Observe(5)

		Convey("It should return a lower rank probability", func() {
			So(value, ShouldBeLessThan, 1)
			So(value, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given ascending samples", testingTB, func() {
		state := RankState{}
		_ = state.Observe(1)
		_ = state.Observe(2)
		rank := state.Observe(3)

		Convey("It should return maximum empirical rank", func() {
			So(rank, ShouldEqual, 1)
		})
	})

	Convey("Given a sample below history", testingTB, func() {
		state := RankState{}
		_ = state.Observe(10)
		_ = state.Observe(20)
		rank := state.Observe(5)

		Convey("It should rank below all history", func() {
			So(rank, ShouldEqual, 1.0/3.0)
		})
	})

	Convey("Given a middle sample", testingTB, func() {
		state := RankState{}
		_ = state.Observe(1)
		_ = state.Observe(3)
		rank := state.Observe(2)

		Convey("It should count at-or-below fraction", func() {
			So(rank, ShouldEqual, 2.0/3.0)
		})
	})
}

func TestRankState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := RankState{}
		samples := []float64{10, 5, 15}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := RankState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func BenchmarkRank_Read(testingTB *testing.B) {
	empirical := NewRank(rankConfig("rank-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	scalarWire(artifact, "sample", 10)
	_ = transport.NewFlipFlop(artifact, empirical)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		scalarWire(artifact, "sample", 10.5)
		_ = transport.NewFlipFlop(artifact, empirical)
	}
}

func BenchmarkRankState_Observe(testingTB *testing.B) {
	state := RankState{}
	_ = state.Observe(10)

	for testingTB.Loop() {
		_ = state.Observe(10.5)
	}
}
