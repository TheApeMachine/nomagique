package statistic_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/statistic"
)

func TestIntegration(t *testing.T) {
	Convey("Given statistic stages composed through nomagique.Number", t, func() {
		Convey("When Panel registers peers before Median excludes the caller", func() {
			panelConfig := datura.Acquire("panel-config", datura.APPJSON).
				Poke("member", "memberKey").
				Poke("sample", "sampleKey")
			medianConfig := datura.Acquire("median-config", datura.APPJSON).
				Poke("member", "memberKey")
			panel := statistic.NewPanel(panelConfig)
			crossSection := nomagique.Number(panel, statistic.NewMedian(medianConfig))

			for _, member := range []struct {
				key   float64
				value float64
			}{
				{1, 0.02},
				{2, 0.04},
				{3, 0.06},
			} {
				artifact := datura.Acquire("test", datura.APPJSON)
				statistic.PanelWire(artifact, member.key, member.value)
				err := nomagique.RoundTripArtifact(artifact, panel)

				So(err, ShouldBeNil)
				artifact.Release()
			}

			artifact := datura.Acquire("test", datura.APPJSON)
			statistic.PanelWire(artifact, 1, 0.02)
			err := nomagique.RoundTripArtifact(artifact, crossSection)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0.05)
			artifact.Release()
		})

		Convey("When Mean streams a uniform series", func() {
			config := datura.Acquire("mean-config", datura.APPJSON).Poke("sample", "input")
			mean := statistic.NewMean(config)
			var lastArtifact *datura.Artifact

			for _, sample := range []float64{1, 2, 3, 4} {
				artifact := datura.Acquire("test", datura.APPJSON)
				statistic.ScalarWire(artifact, "sample", sample)
				err := nomagique.RoundTripArtifact(artifact, mean)

				So(err, ShouldBeNil)

				if lastArtifact != nil {
					lastArtifact.Release()
				}

				lastArtifact = artifact
			}

			defer lastArtifact.Release()
			So(datura.Peek[float64](lastArtifact, "output", "value"), ShouldEqual, 2.5)
		})
	})
}
