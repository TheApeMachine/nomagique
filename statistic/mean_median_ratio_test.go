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
			Poke(0.0, "stageIndex").
			Poke([]string{"rvol"}, "order").
			Poke(map[string]any{
				"input":       "volume",
				"shortWindow": 3.0,
				"longWindow":  5.0,
				"outputKey":   "rvol",
			}, "rvol")

		stage := NewMeanMedianRatio(config)
		var lastFrame *datura.Artifact

		for _, sample := range []float64{1, 1, 1, 1, 10} {
			frame := datura.Acquire("mean-median-ratio-test-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume"})
			frame.Merge("features", []float64{sample})

			err := transport.NewFlipFlop(frame, stage)

			if len(stage.histories["rvol"]) < 5 {
				So(err, ShouldNotBeNil)
			}

			if len(stage.histories["rvol"]) >= 5 {
				So(err, ShouldBeNil)
			}

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

	Convey("Given disjoint short and long windows", testingTB, func() {
		config := datura.Acquire("mean-median-ratio-disjoint-config", datura.APPJSON).
			Poke(0.0, "stageIndex").
			Poke([]string{"rvol"}, "order").
			Poke(map[string]any{
				"input":       "volume",
				"shortWindow": 2.0,
				"longWindow":  5.0,
				"outputKey":   "rvol",
			}, "rvol")

		stage := NewMeanMedianRatio(config)
		samples := []float64{10, 20, 30, 40, 1000}
		var lastFrame *datura.Artifact

		for _, sample := range samples {
			frame := datura.Acquire("mean-median-ratio-disjoint-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume"})
			frame.Merge("features", []float64{sample})

			err := transport.NewFlipFlop(frame, stage)

			if len(stage.histories["rvol"]) < 5 {
				So(err, ShouldNotBeNil)
			}

			if len(stage.histories["rvol"]) >= 5 {
				So(err, ShouldBeNil)
			}

			if lastFrame != nil {
				lastFrame.Release()
			}

			lastFrame = frame
		}

		defer lastFrame.Release()

		Convey("It should exclude short-window samples from the long median", func() {
			shortMean := (40.0 + 1000.0) / 2.0
			longMedian := 20.0

			So(
				datura.Peek[float64](lastFrame, "output", "rvol"),
				ShouldAlmostEqual,
				shortMean/longMedian,
				1e-9,
			)
		})
	})

	Convey("Given delta transform with decline output configured", testingTB, func() {
		config := datura.Acquire("mean-median-ratio-decline-config", datura.APPJSON).
			Poke(0.0, "stageIndex").
			Poke([]string{"lift"}, "order").
			Poke(map[string]any{
				"input":       "volume",
				"transform":   "deltaPositive",
				"shortWindow": 1.0,
				"longWindow":  2.0,
				"outputKey":   "lift",
				"decline": map[string]any{
					"output": "liftDecline",
				},
			}, "lift")

		stage := NewMeanMedianRatio(config)
		var lastFrame *datura.Artifact

		for _, sample := range []float64{100, 200, 50} {
			frame := datura.Acquire("mean-median-ratio-decline-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume"})
			frame.Merge("features", []float64{sample})

			err := transport.NewFlipFlop(frame, stage)

			if len(stage.histories["lift"]) < 2 {
				So(err, ShouldNotBeNil)
			}

			if len(stage.histories["lift"]) >= 2 {
				So(err, ShouldBeNil)
			}

			if lastFrame != nil {
				lastFrame.Release()
			}

			lastFrame = frame
		}

		defer lastFrame.Release()

		Convey("It should publish decline from configured output key", func() {
			So(datura.Peek[float64](lastFrame, "output", "liftDecline"), ShouldBeGreaterThan, 0)
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
	})

	Convey("Given dynamic windows on the first sample", testingTB, func() {
		config := datura.Acquire("mean-median-ratio-dynamic-config", datura.APPJSON).
			Poke(0.0, "stageIndex").
			Poke([]string{"rvol"}, "order").
			Poke(map[string]any{
				"input":       "volume",
				"shortWindow": 0.0,
				"longWindow":  0.0,
				"outputKey":   "rvol",
			}, "rvol")

		stage := NewMeanMedianRatio(config)
		artifact := datura.Acquire("mean-median-ratio-dynamic-test", datura.APPJSON)
		artifact.Merge("root", "features")
		artifact.Merge("inputs", []string{"volume"})
		artifact.Merge("features", []float64{10})

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldNotBeNil)
	})
}

func BenchmarkMeanMedianRatioRead(b *testing.B) {
	config := datura.Acquire("mean-median-ratio-bench", datura.APPJSON).
		Poke(0.0, "stageIndex").
		Poke([]string{"rvol"}, "order").
		Poke(map[string]any{
			"input":       "volume",
			"shortWindow": 3.0,
			"longWindow":  5.0,
			"outputKey":   "rvol",
		}, "rvol")

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
