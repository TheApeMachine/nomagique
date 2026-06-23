package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func precursorConfig() *datura.Artifact {
	return datura.Acquire("precursor-config", datura.APPJSON).
		Poke([]string{"rvol", "precursor"}, "order").
		Poke(1.0, "stageIndex").
		Poke(map[string]any{
			"input":      "last",
			"returnLag":  1.0,
			"longWindow": 5.0,
			"outputKey":  "precursor",
			"stageIndex": 1.0,
		}, "precursor")
}

func precursorState(last float64) *datura.Artifact {
	artifact := datura.Acquire("precursor-state", datura.APPJSON)
	artifact.Merge("root", "features")
	artifact.Merge("inputs", []string{"volume", "last"})
	artifact.Merge("features", []float64{100, last})

	return artifact
}

func TestPriceRingRead(t *testing.T) {
	Convey("Given a price ring stage", t, func() {
		config := precursorConfig()
		stage := NewPriceRing(config)
		var lastArtifact *datura.Artifact

		for _, last := range []float64{100, 101, 102} {
			artifact := precursorState(last)
			err := transport.NewFlipFlop(artifact, stage)
			So(err, ShouldBeNil)
			lastArtifact = artifact
		}

		Convey("It should publish the current sample on the outbound wire", func() {
			So(datura.Peek[string](lastArtifact, "root"), ShouldEqual, "output")
			So(datura.Peek[float64](lastArtifact, "output", "last"), ShouldEqual, 102)
		})
	})
}

func BenchmarkPriceRingRead(b *testing.B) {
	config := precursorConfig()
	stage := NewPriceRing(config)

	b.ReportAllocs()

	for b.Loop() {
		artifact := datura.Acquire("precursor-state", datura.APPJSON)
		artifact.Poke("features", "root")
		artifact.Poke([]string{"volume", "last"}, "inputs")
		artifact.WithPayload(datura.Map[any]{
			"features": []float64{100, 101},
		}.Marshal())
		_ = transport.NewFlipFlop(artifact, stage)
		artifact.Release()
	}
}
