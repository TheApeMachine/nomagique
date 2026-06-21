package equation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/vector"
)

func ignitionReplayConfig() *datura.Artifact {
	return datura.Acquire("pumpdump-ignition-replay", datura.APPJSON).
		Poke(0.0, "stageIndex").
		Poke([]string{"rvol", "precursor", "compression"}, "order").
		Poke([]string{"ignition", "compression", "trend", "exhaustion"}, "outputs").
		Poke(0.0, "threshold").
		Poke(map[string]any{
			"rvol": map[string]any{
				"input":       "volume",
				"transform":   "deltaPositive",
				"shortWindow": 0.0,
				"longWindow":  0.0,
				"outputKey":   "rvol",
				"scale":       0.0,
				"decline": map[string]any{
					"output": "rvolDecline",
				},
			},
			"precursor": map[string]any{
				"input":        "last",
				"returnLag":    1.0,
				"longWindow":   0.0,
				"positiveOnly": 1.0,
				"outputKey":    "precursor",
				"stageIndex":   1.0,
				"scale":        0.0,
			},
			"compression": map[string]any{
				"input":  "spread",
				"source": "value",
				"scale":  0.0,
			},
			"spread": map[string]any{
				"inputs": []string{"bid", "ask"},
			},
			"joint": map[string]any{
				"leftKey":        "rvol",
				"rightKey":       "precursor",
				"destinationKey": "ignition",
				"source":         "ignition",
				"output":         "ignition",
			},
		}, "inputs")
}

func TestIgnitionSpreadAfterLogReturn(testingTB *testing.T) {
	Convey("Given features after log-return z-score in ignition", testingTB, func() {
		config := ignitionReplayConfig()
		stage := transport.NewPipeline(
			statistic.NewMeanMedianRatio(config),
			NewLogReturnZScore(config),
			vector.NewSpreadSample(config),
		)
		frame := datura.Acquire("ignition-spread-pipeline-frame", datura.APPJSON)
		frame.Merge("root", "features")
		frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		frame.Merge("features", []float64{120, 10050, 10050.0001, 10050.0002})

		err := transport.NewFlipFlop(frame, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "spread"), ShouldBeGreaterThan, 0)
	})
}

func TestIgnitionSpreadOutput(testingTB *testing.T) {
	Convey("Given bid and ask features through ignition", testingTB, func() {
		config := ignitionReplayConfig()
		stage := NewIgnition(config)
		frame := datura.Acquire("ignition-spread-frame", datura.APPJSON)
		frame.Merge("root", "features")
		frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		frame.Merge("features", []float64{120, 10050, 10050.0001, 10050.0002})

		err := transport.NewFlipFlop(frame, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "spread"), ShouldBeGreaterThan, 0)
	})
}

func TestIgnitionReplayTraversal(testingTB *testing.T) {
	Convey("Given a long replay on one shared ignition config", testingTB, func() {
		config := ignitionReplayConfig()
		stage := NewIgnition(config)
		var artifact *datura.Artifact

		for tick := range 120 {
			volume := 100.0 + float64(tick)
			last := 10000.0 + float64(tick)*10
			frame := datura.Acquire("ignition-replay-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
			frame.Merge("features", []float64{volume, last, last - 1, last + 1})

			err := transport.NewFlipFlop(frame, stage)

			So(err, ShouldBeNil)

			if artifact != nil {
				artifact.Release()
			}

			artifact = frame
		}

		defer artifact.Release()

		Convey("It should still publish ignition logits after replay", func() {
			So(datura.Peek[float64](artifact, "output", "rvol"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "ignition"), ShouldBeGreaterThanOrEqualTo, 0)
		})
	})
}

func BenchmarkIgnitionReplayTraversal(b *testing.B) {
	config := ignitionReplayConfig()
	stage := NewIgnition(config)

	b.ReportAllocs()

	for b.Loop() {
		var lastFrame *datura.Artifact

		for tick := range 120 {
			volume := 100.0 + float64(tick)
			last := 10000.0 + float64(tick)*10
			frame := datura.Acquire("ignition-replay-bench-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
			frame.Merge("features", []float64{volume, last, last - 1, last + 1})

			if err := transport.NewFlipFlop(frame, stage); err != nil {
				b.Fatal(err)
			}

			if lastFrame != nil {
				lastFrame.Release()
			}

			lastFrame = frame
		}

		if lastFrame != nil {
			lastFrame.Release()
		}
	}
}
