package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestNewCUSUM(testingTB *testing.T) {
	Convey("Given CUSUM constructor", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))

		Convey("It should return a usable dynamic", func() {
			So(changeSum, ShouldNotBeNil)
		})
	})
}

func TestCUSUMRead(testingTB *testing.T) {
	Convey("Given empty inbound wire", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := nomagique.RoundTripArtifact(artifact, changeSum)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a first sample with zero span", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		err := nomagique.RoundTripArtifact(artifact, changeSum)

		Convey("It should return a span error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a warmed change sum", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		_ = nomagique.RoundTripArtifact(artifact, changeSum)

		scalarWire(artifact, "sample", 25)
		err := nomagique.RoundTripArtifact(artifact, changeSum)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should accumulate evidence", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given sequential scalar samples", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		_ = nomagique.RoundTripArtifact(artifact, changeSum)

		scalarWire(artifact, "sample", 8)
		err := nomagique.RoundTripArtifact(artifact, changeSum)

		So(err, ShouldBeNil)

		withWork := datura.Peek[float64](artifact, "output", "value")

		combined := NewCUSUM(cusumConfig("cusum-config-combined"))
		reference := datura.Acquire("test", datura.APPJSON)

		scalarWire(reference, "sample", 10)
		_ = nomagique.RoundTripArtifact(reference, combined)

		scalarWire(reference, "sample", 8)
		err = nomagique.RoundTripArtifact(reference, combined)

		So(err, ShouldBeNil)

		direct := datura.Peek[float64](reference, "output", "value")

		Convey("It should match a single combined scalar", func() {
			So(withWork, ShouldEqual, direct)
		})
	})

	Convey("Given reset after observation", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)

		_ = nomagique.RoundTripArtifact(artifact, changeSum)

		scalarWire(artifact, "sample", 25)
		_ = nomagique.RoundTripArtifact(artifact, changeSum)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err := nomagique.RoundTripArtifact(resetArtifact, changeSum)

		So(err, ShouldNotBeNil)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](changeSum.artifact, "output", "count"), ShouldEqual, 0)
			So(datura.Peek[float64](changeSum.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkCUSUMRead(testingTB *testing.B) {
	changeSum := NewCUSUM(cusumConfig("cusum-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	scalarWire(artifact, "sample", 10)
	_ = nomagique.RoundTripArtifact(artifact, changeSum)
	scalarWire(artifact, "sample", 10.5)
	_ = nomagique.RoundTripArtifact(artifact, changeSum)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		scalarWire(artifact, "sample", 10.5)
		_ = nomagique.RoundTripArtifact(artifact, changeSum)
	}
}
