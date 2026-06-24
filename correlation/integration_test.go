package correlation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/correlation"
)

func TestIntegration(t *testing.T) {
	Convey("Given correlation stages composed through nomagique.Number", t, func() {
		Convey("When Pearson receives a perfectly correlated batch", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke("data", "root").
				Poke([]string{"batch"}, "inputs").
				Poke([]float64{1, 2, 1, 2}, "data", "batch")
			pipeline := nomagique.Number(
				correlation.NewPearson(datura.Acquire("pearson-config", datura.APPJSON).
					Poke("batch", "input")),
			)
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		})

		Convey("When IntervalSeries streams epoch and level pairs", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			series := nomagique.Number(
				correlation.NewIntervalSeries(correlation.IntervalWireConfig("interval-series-config")),
			)

			artifact = correlation.EpochLevelWire(artifact, float64(1_000), 100.0)
			err := transport.NewFlipFlop(artifact, series)

			So(err, ShouldNotBeNil)

			artifact = correlation.EpochLevelWire(artifact, float64(2_000), 110.0)
			err = transport.NewFlipFlop(artifact, series)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})

		Convey("When Contagion runs on correlated members", func() {
			contagion := nomagique.Number(
				correlation.NewContagion(
					datura.Acquire("test", datura.APPJSON).
						WithAttributes(datura.Map[any]{
							"memberKey": "member",
							"sampleKey": "sample",
							"pairedKey": "paired",
							"config": datura.Map[any]{
								"minSamples":    2.0,
								"memberCap":     2.0,
								"adaptiveSigma": 2.0,
								"tier": datura.Map[any]{
									"fast":   4.0,
									"medium": 8.0,
									"slow":   16.0,
								},
							},
						}),
				),
			)
			artifact := datura.Acquire("test", datura.APPJSON)

			for step := range 16 {
				epoch := float64((step + 1) * 1_000)
				artifact.Poke("wire", "root")
				artifact.Poke([]string{"member", "sample", "paired"}, "inputs")
				artifact.Merge("wire", map[string]any{
					"member": 1,
					"sample": epoch,
					"paired": 100 + float64(step)*0.1,
				})
				err := transport.NewFlipFlop(artifact, contagion)

				if step == 0 {
					So(err, ShouldNotBeNil)
				}

				artifact.Poke("wire", "root")
				artifact.Poke([]string{"member", "sample", "paired"}, "inputs")
				artifact.Merge("wire", map[string]any{
					"member": 2,
					"sample": epoch,
					"paired": 50 + float64(step)*0.05,
				})
				err = transport.NewFlipFlop(artifact, contagion)

				if step == 0 {
					So(err, ShouldNotBeNil)
				}
			}

			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}
