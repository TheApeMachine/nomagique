package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestEntropySeries(t *testing.T) {
	Convey("Given an Entropy stage", t, func() {
		uniformArtifact := datura.Acquire("test", datura.APPJSON)
		uniformStage := NewEntropy(0)

		for _, sample := range []float64{1, 1, 1, 1} {
			uniformArtifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(uniformArtifact, uniformStage)

			So(err, ShouldBeNil)
		}

		uniform := datura.Peek[float64](uniformArtifact, "output", "value")

		peakedArtifact := datura.Acquire("test", datura.APPJSON)
		peakedStage := NewEntropy(0)

		for _, sample := range []float64{100, 1, 1, 1} {
			peakedArtifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(peakedArtifact, peakedStage)

			So(err, ShouldBeNil)
		}

		peaked := datura.Peek[float64](peakedArtifact, "output", "value")

		Convey("It should rank peaked mass lower than uniform mass", func() {
			So(peaked, ShouldBeLessThan, uniform)
		})
	})
}
