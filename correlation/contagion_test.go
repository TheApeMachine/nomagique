package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func contagionConfigArtifact() *datura.Artifact {
	return datura.Acquire("test", datura.APPJSON).
		WithAttributes(datura.Map[any]{
			"memberKey": "member",
			"sampleKey": "sample",
			"pairedKey": "paired",
			"config": datura.Map[any]{
				"minSamples":    1.0,
				"memberCap":     2.0,
				"adaptiveSigma": 2.0,
				"tier": datura.Map[any]{
					"fast":   8.0,
					"medium": 8.0,
					"slow":   8.0,
				},
			},
		})
}

func contagionWire(
	artifact *datura.Artifact,
	member int,
	sample float64,
	paired float64,
) *datura.Artifact {
	artifact.Poke("wire", "root")
	artifact.Poke([]string{"member", "sample", "paired"}, "inputs")
	artifact.Merge("wire", map[string]any{
		"member": member,
		"sample": sample,
		"paired": paired,
	})

	return artifact
}

func TestMedianPairwiseAbsCorrelation(testingTB *testing.T) {
	Convey("Given proportional members in contagion", testingTB, func() {
		contagion := NewContagion(contagionConfigArtifact())
		artifact := datura.Acquire("test", datura.APPJSON)

		contagionWire(artifact, 1, float64(1_000), 100.0)
		err := transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldNotBeNil)

		contagionWire(artifact, 1, float64(2_000), 110.0)
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		contagionWire(artifact, 2, float64(1_000), 50.0)
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		contagionWire(artifact, 2, float64(2_000), 55.0)
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		Convey("It should return unit median correlation", func() {
			value := datura.Peek[float64](artifact, "output", "tier.fast")
			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func TestContagionRead(testingTB *testing.T) {
	Convey("Given a contagion stage with fed members", testingTB, func() {
		contagion := NewContagion(contagionConfigArtifact())
		artifact := datura.Acquire("test", datura.APPJSON)

		contagionWire(artifact, 1, float64(1_000), 100.0)
		err := transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldNotBeNil)

		contagionWire(artifact, 1, float64(2_000), 110.0)
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		contagionWire(artifact, 2, float64(1_000), 50.0)
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		contagionWire(artifact, 2, float64(2_000), 55.0)
		err = transport.NewFlipFlop(artifact, contagion)

		So(err, ShouldBeNil)

		value := datura.Peek[float64](artifact, "output", "value")

		Convey("It should publish positive coupling for correlated tiers", func() {
			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkContagionRead(testingTB *testing.B) {
	contagion := NewContagion(
		datura.Acquire("test", datura.APPJSON).
			WithAttributes(datura.Map[any]{
				"memberKey": "member",
				"sampleKey": "sample",
				"pairedKey": "paired",
				"config": datura.Map[any]{
					"minSamples": 8.0,
					"memberCap":  16.0,
					"tier": datura.Map[any]{
						"fast":   8.0,
						"medium": 16.0,
						"slow":   32.0,
					},
				},
			}),
	)
	artifact := datura.Acquire("test", datura.APPJSON)

	for member := range 16 {
		for step := range 32 {
			contagionWire(
				artifact,
				member+1,
				float64((step+1)*1_000),
				100+float64(member)+float64(step)*0.01,
			)
			_ = transport.NewFlipFlop(artifact, contagion)
		}
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, contagion)
	}
}
