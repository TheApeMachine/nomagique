package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestNewRank(testingTB *testing.T) {
	Convey("Given Rank constructor", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))

		Convey("It should return a usable dynamic", func() {
			So(empirical, ShouldNotBeNil)
		})
	})
}

func TestRankRead(testingTB *testing.T) {
	Convey("Given empty inbound wire", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := nomagique.RoundTripArtifact(artifact, empirical)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given empirical rank history", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		err := nomagique.RoundTripArtifact(artifact, empirical)

		So(err, ShouldBeNil)

		scalarWire(artifact, "sample", 5)
		err = nomagique.RoundTripArtifact(artifact, empirical)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return the empirical probability at or below the sample", func() {
			So(got, ShouldEqual, 0.5)
		})
	})

	Convey("Given sub-unit empirical rank history", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{0.1, 0.2, 0.15} {
			scalarWire(artifact, "sample", sample)
			err := nomagique.RoundTripArtifact(artifact, empirical)

			So(err, ShouldBeNil)
		}

		value := datura.Peek[float64](artifact, "output", "value")
		count := datura.Peek[float64](empirical.artifact, "output", "count")
		history := datura.Peek[[]float64](empirical.artifact, "history")

		Convey("It should retain observations by count instead of value span", func() {
			So(value, ShouldAlmostEqual, 2.0/3.0)
			So(count, ShouldEqual, 3)
			So(len(history), ShouldEqual, 3)
		})
	})

	Convey("Given equal consecutive scalar samples", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{10, 10} {
			scalarWire(artifact, "sample", sample)
			err := nomagique.RoundTripArtifact(artifact, empirical)

			So(err, ShouldBeNil)
		}

		Convey("It should still advance the observation count", func() {
			So(datura.Peek[float64](empirical.artifact, "output", "count"), ShouldEqual, 2)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		})
	})

	Convey("Given sequential scalar samples", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		err := nomagique.RoundTripArtifact(artifact, empirical)

		So(err, ShouldBeNil)

		scalarWire(artifact, "sample", 8)
		err = nomagique.RoundTripArtifact(artifact, empirical)

		So(err, ShouldBeNil)

		withWork := datura.Peek[float64](artifact, "output", "value")

		combined := NewRank(rankConfig("rank-config-combined"))
		reference := datura.Acquire("test", datura.APPJSON)

		scalarWire(reference, "sample", 10)
		err = nomagique.RoundTripArtifact(reference, combined)

		So(err, ShouldBeNil)

		scalarWire(reference, "sample", 8)
		err = nomagique.RoundTripArtifact(reference, combined)

		So(err, ShouldBeNil)

		direct := datura.Peek[float64](reference, "output", "value")

		Convey("It should match a single combined scalar", func() {
			So(withWork, ShouldEqual, direct)
		})
	})

	Convey("Given reset after observation", testingTB, func() {
		empirical := NewRank(rankConfig("rank-config"))
		artifact := scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)

		err := nomagique.RoundTripArtifact(artifact, empirical)

		So(err, ShouldBeNil)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err = nomagique.RoundTripArtifact(resetArtifact, empirical)

		So(err, ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](resetArtifact, "output", "count"), ShouldEqual, 0)
			So(datura.Peek[float64](resetArtifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkRankRead(testingTB *testing.B) {
	empirical := NewRank(rankConfig("rank-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	scalarWire(artifact, "sample", 10)
	_ = nomagique.RoundTripArtifact(artifact, empirical)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		scalarWire(artifact, "sample", 10.5)
		_ = nomagique.RoundTripArtifact(artifact, empirical)
	}
}
