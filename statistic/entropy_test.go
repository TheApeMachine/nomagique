package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestEntropyRead(t *testing.T) {
	Convey("Given an Entropy stage", t, func() {
		uniformArtifact := datura.Acquire("test", datura.APPJSON)
		uniformStage := NewEntropy(scalarStageConfig("entropy-config-uniform"))

		for _, sample := range []float64{1, 1, 1, 1} {
			err := transport.NewFlipFlop(ScalarWire(uniformArtifact, "sample", sample), uniformStage)

			So(err, ShouldBeNil)
		}

		uniform := datura.Peek[float64](uniformArtifact, "output", "value")

		peakedArtifact := datura.Acquire("test", datura.APPJSON)
		peakedStage := NewEntropy(scalarStageConfig("entropy-config-peaked"))

		for _, sample := range []float64{100, 1, 1, 1} {
			err := transport.NewFlipFlop(ScalarWire(peakedArtifact, "sample", sample), peakedStage)

			So(err, ShouldBeNil)
		}

		peaked := datura.Peek[float64](peakedArtifact, "output", "value")

		Convey("It should rank peaked mass lower than uniform mass", func() {
			So(peaked, ShouldBeLessThan, uniform)
		})
	})
}
