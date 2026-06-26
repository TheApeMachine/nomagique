package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestIntervalCouplingRead(testingTB *testing.T) {
	Convey("Given proportional interval histories", testingTB, func() {
		coupling := NewIntervalCoupling(IntervalWireConfig("interval-coupling-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(0, "config", "side")
		artifact = EpochLevelWire(artifact, float64(1_000), 100.0)
		err := nomagique.RoundTripArtifact(artifact, coupling)

		So(err, ShouldNotBeNil)

		artifact.Poke(0, "config", "side")
		artifact = EpochLevelWire(artifact, float64(2_000), 110.0)
		err = nomagique.RoundTripArtifact(artifact, coupling)

		So(err, ShouldNotBeNil)

		artifact.Poke(1, "config", "side")
		artifact = EpochLevelWire(artifact, float64(1_000), 50.0)
		err = nomagique.RoundTripArtifact(artifact, coupling)

		So(err, ShouldNotBeNil)

		artifact.Poke(1, "config", "side")
		artifact = EpochLevelWire(artifact, float64(2_000), 55.0)
		err = nomagique.RoundTripArtifact(artifact, coupling)

		So(err, ShouldBeNil)

		value := datura.Peek[float64](artifact, "output", "value")

		Convey("It should estimate unit correlation", func() {
			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func BenchmarkIntervalCouplingRead(testingTB *testing.B) {
	coupling := NewIntervalCoupling(IntervalWireConfig("interval-coupling-config"))
	artifact := datura.Acquire("test", datura.APPJSON)

	for step := range 64 {
		epoch := float64((step + 1) * 1_000)
		artifact.Poke(0, "config", "side")
		artifact = EpochLevelWire(artifact, epoch, 100+float64(step)*0.1)
		_ = nomagique.RoundTripArtifact(artifact, coupling)
		artifact.Poke(1, "config", "side")
		artifact = EpochLevelWire(artifact, epoch, 50+float64(step)*0.05)
		_ = nomagique.RoundTripArtifact(artifact, coupling)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = nomagique.RoundTripArtifact(artifact, coupling)
	}
}
