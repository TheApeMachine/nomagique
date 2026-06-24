package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestMaxSeries(t *testing.T) {
	Convey("Given a Max stage", t, func() {
		config := datura.Acquire("max-config", datura.APPJSON).Poke("sample", "input")
		maxStage := NewMax(config)
		var lastArtifact *datura.Artifact

		for _, sample := range []float64{3, 1, 2} {
			artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", sample)
			err := transport.NewFlipFlop(artifact, maxStage)
			So(err, ShouldBeNil)

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		defer lastArtifact.Release()

		got := datura.Peek[float64](lastArtifact, "output", "value")

		Convey("It should retain the maximum", func() {
			So(got, ShouldEqual, 3)
		})
	})
}
