package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestMeanMedianRatioRead(testingTB *testing.T) {
	Convey("Given configured windows on the artifact", testingTB, func() {
		config := datura.Acquire("mean-median-ratio-config", datura.APPJSON).
			Poke([]string{"rvol"}, "order").
			Poke(map[string]any{
				"rvol": map[string]any{
					"input":       "volume",
					"useDelta":    0.0,
					"shortWindow": 3.0,
					"longWindow":  5.0,
					"outputKey":   "rvol",
				},
			}, "inputs")

		stage := NewMeanMedianRatio(config)
		artifact := datura.Acquire("mean-median-ratio-test", datura.APPJSON).
			Poke("features", "root").
			Poke([]string{"volume"}, "inputs")

		for _, sample := range []float64{1, 1, 1, 1, 10} {
			artifact.Poke([]float64{sample}, "features")
			err := transport.NewFlipFlop(artifact, stage)
			So(err, ShouldBeNil)
		}

		Convey("It should publish the short mean over long median ratio", func() {
			So(datura.Peek[float64](artifact, "output", "rvol"), ShouldBeGreaterThan, 1)
		})
	})

	Convey("Given missing window configuration", testingTB, func() {
		config := datura.Acquire("mean-median-ratio-empty", datura.APPJSON)
		stage := NewMeanMedianRatio(config)
		artifact := datura.Acquire("mean-median-ratio-empty-test", datura.APPJSON).
			Poke("features", "root").
			Poke([]string{"volume"}, "inputs")
		artifact.Poke([]float64{10}, "features")

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "rvol"), ShouldEqual, 0)
	})
}

func BenchmarkMeanMedianRatioRead(b *testing.B) {
	config := datura.Acquire("mean-median-ratio-bench", datura.APPJSON).
		Poke([]string{"rvol"}, "order").
		Poke(map[string]any{
			"rvol": map[string]any{
				"input":       "volume",
				"useDelta":    0.0,
				"shortWindow": 3.0,
				"longWindow":  5.0,
				"outputKey":   "rvol",
			},
		}, "inputs")

	stage := NewMeanMedianRatio(config)
	artifact := datura.Acquire("mean-median-ratio-bench-test", datura.APPJSON).
		Poke("features", "root").
		Poke([]string{"volume"}, "inputs")

	b.ReportAllocs()

	for b.Loop() {
		artifact.Poke([]float64{10}, "features")
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
