package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestCUSUM(testingTB *testing.T) {
	Convey("Given CUSUM constructor", testingTB, func() {
		changeSum := NewCUSUM(datura.Acquire("cusum-config", datura.APPJSON))

		Convey("It should return a usable dynamic", func() {
			So(changeSum, ShouldNotBeNil)
		})
	})
}

func TestChangeSum_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		changeSum := NewCUSUM(datura.Acquire("cusum-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})

	Convey("Given a change sum", testingTB, func() {
		changeSum := NewCUSUM(datura.Acquire("cusum-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(10, "sample")
		err := transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		artifact.Poke(25, "sample")
		err = transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should accumulate evidence", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a combined scalar sample", testingTB, func() {
		changeSum := NewCUSUM(datura.Acquire("cusum-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(10, "sample")
		err := transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		artifact.Poke(8, "sample")
		err = transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		withWork := datura.Peek[float64](artifact, "output", "value")

		combined := NewCUSUM(datura.Acquire("cusum-config-combined", datura.APPJSON))
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

func TestChangeSum_Reset(testingTB *testing.T) {
	Convey("Given an observed change sum", testingTB, func() {
		changeSum := NewCUSUM(datura.Acquire("cusum-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample")

		err := transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err = transport.NewFlipFlop(resetArtifact, changeSum)

		So(err, ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](resetArtifact, "output", "ready"), ShouldEqual, 0)
			So(datura.Peek[float64](resetArtifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkCUSUM_Observe(testingTB *testing.B) {
	changeSum := NewCUSUM(datura.Acquire("cusum-config-bench", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact.Poke(10, "sample")
	_ = transport.NewFlipFlop(artifact, changeSum)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke(10.5, "sample")
		_ = transport.NewFlipFlop(artifact, changeSum)
	}
}
