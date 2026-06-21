package learning_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/learning"
)

func TestIntegration(t *testing.T) {
	Convey("Given learning stages composed through nomagique.Number", t, func() {
		Convey("When Weight observes a matched prediction", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(10, "sample").
				Poke(10, "paired")
			pipeline := nomagique.Number(learning.Weight())
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		})

		Convey("When SampleRatio and Forecast run in sequence", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			pipeline := nomagique.Number(learning.SampleRatio(), learning.Forecast())

			artifact.Poke(10, "sample").Poke(10, "paired")
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)

			artifact.Poke(10, "sample").Poke(15, "paired")
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
