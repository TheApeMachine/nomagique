package learning_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/learning"
)

func pairConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "sampleKey").
		Poke("paired", "pairedKey")
}

func pairWire(artifact *datura.Artifact, predicted float64, actual float64) *datura.Artifact {
	artifact.Poke("wire", "root")
	artifact.Poke([]string{"sample", "paired"}, "inputs")
	artifact.Merge("wire", map[string]any{
		"sample": predicted,
		"paired": actual,
	})

	return artifact
}

func TestIntegration(t *testing.T) {
	Convey("Given learning stages composed through nomagique.Number", t, func() {
		Convey("When Weight observes a matched prediction", func() {
			artifact := pairWire(datura.Acquire("test", datura.APPJSON), 10, 10)
			pipeline := nomagique.Number(learning.Weight(pairConfig("trust-weight-config")))
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		})

		Convey("When SampleRatio and Forecast run in sequence", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			pipeline := nomagique.Number(
				learning.SampleRatio(pairConfig("sample-ratio-config")),
				learning.Forecast(datura.Acquire("forecast-config", datura.APPJSON).
					Poke("predicted", "sampleKey").
					Poke("actual", "pairedKey")),
			)

			artifact = pairWire(artifact, 10, 10)
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)

			artifact = pairWire(artifact, 10, 15)
			err = transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 1)
		})

		Convey("When RLS ingests feature and target batch", func() {
			stage, err := learning.NewRLS(1, 1000)

			So(err, ShouldBeNil)

			artifact := datura.Acquire("test", datura.APPJSON).
				Poke([]float64{2, 4}, "batch")
			err = transport.NewFlipFlop(artifact, nomagique.Number(stage))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}
