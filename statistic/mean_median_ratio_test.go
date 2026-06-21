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
		var lastFrame *datura.Artifact

		for _, sample := range []float64{1, 1, 1, 1, 10} {
			frame := datura.Acquire("mean-median-ratio-test-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume"})
			frame.Merge("features", []float64{sample})

			err := transport.NewFlipFlop(frame, stage)

			So(err, ShouldBeNil)

			if lastFrame != nil {
				lastFrame.Release()
			}

			lastFrame = frame
		}

		defer lastFrame.Release()

		Convey("It should publish the short mean over long median ratio", func() {
			So(datura.Peek[float64](lastFrame, "output", "rvol"), ShouldBeGreaterThan, 1)
		})
	})

	Convey("Given missing window configuration", testingTB, func() {
		config := datura.Acquire("mean-median-ratio-empty", datura.APPJSON)
		stage := NewMeanMedianRatio(config)
		artifact := datura.Acquire("mean-median-ratio-empty-test", datura.APPJSON)
		artifact.Merge("root", "features")
		artifact.Merge("inputs", []string{"volume"})
		artifact.Merge("features", []float64{10})

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
	artifact := datura.Acquire("mean-median-ratio-bench-test", datura.APPJSON)
	artifact.Merge("root", "features")
	artifact.Merge("inputs", []string{"volume"})

	b.ReportAllocs()

	for b.Loop() {
		artifact.Merge("features", []float64{10})
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
