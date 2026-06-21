package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func contagionConfigArtifact() *datura.Artifact {
	return datura.Acquire("test", datura.APPJSON).
		Poke(1, "config", "minSamples").
		Poke(2, "config", "memberCap").
		Poke(2, "config", "adaptiveSigma").
		Poke(8, "config", "tier", "fast").
		Poke(8, "config", "tier", "medium").
		Poke(8, "config", "tier", "slow")
}

func TestMedianPairwiseAbsCorrelation(testingTB *testing.T) {
	Convey("Given proportional members in contagion", testingTB, func() {
		contagion := NewContagion(contagionConfigArtifact())
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(1, "member").Poke(float64(1_000), "sample").Poke(100.0, "paired")
		err := transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		artifact.Poke(1, "member").Poke(float64(2_000), "sample").Poke(110.0, "paired")
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		artifact.Poke(2, "member").Poke(float64(1_000), "sample").Poke(50.0, "paired")
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		artifact.Poke(2, "member").Poke(float64(2_000), "sample").Poke(55.0, "paired")
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		Convey("It should return unit median correlation", func() {
			value := datura.Peek[float64](artifact, "output", "tier.fast")
			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func TestContagionObserve(testingTB *testing.T) {
	Convey("Given a contagion stage with fed members", testingTB, func() {
		contagion := NewContagion(contagionConfigArtifact())
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(1, "member").Poke(float64(1_000), "sample").Poke(100.0, "paired")
		err := transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		artifact.Poke(1, "member").Poke(float64(2_000), "sample").Poke(110.0, "paired")
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		artifact.Poke(2, "member").Poke(float64(1_000), "sample").Poke(50.0, "paired")
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		artifact.Poke(2, "member").Poke(float64(2_000), "sample").Poke(55.0, "paired")
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		value := datura.Peek[float64](artifact, "output", "value")

		Convey("It should publish positive coupling for correlated tiers", func() {
			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkContagionObserve(testingTB *testing.B) {
	contagion := NewContagion(
		datura.Acquire("test", datura.APPJSON).
			Poke(8, "config", "minSamples").
			Poke(16, "config", "memberCap").
			Poke(8, "config", "tier", "fast").
			Poke(16, "config", "tier", "medium").
			Poke(32, "config", "tier", "slow"),
	)
	artifact := datura.Acquire("test", datura.APPJSON)

	for member := range 16 {
		for step := range 32 {
			artifact.Poke(float64(member+1), "member").
				Poke(float64((step+1)*1_000), "sample").
				Poke(100+float64(member)+float64(step)*0.01, "paired")
			_ = transport.NewFlipFlop(artifact, contagion)
		}
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, contagion)
	}
}
