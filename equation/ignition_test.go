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
		Poke("output", "root").
		Poke([]string{"rvol", "precursor", "compression", "spread", "ignition", "value", "rvolDecline"}, "inputs").
		Poke(0.0, "stageIndex").
		Poke([]string{"rvol", "precursor", "compression"}, "order").
		Poke([]string{"ignition", "compression", "trend", "exhaustion"}, "outputs").
		Poke(0.01, "threshold").
		Poke(map[string]any{
			"input":       "volume",
			"transform":   "deltaPositive",
			"shortWindow": 0.0,
			"longWindow":  0.0,
			"outputKey":   "rvol",
			"scale":       0.0,
			"decline": map[string]any{
				"output": "rvolDecline",
			},
		}, "rvol").
		Poke(map[string]any{
			"input":        "last",
			"returnLag":    1.0,
			"longWindow":   0.0,
			"positiveOnly": 1.0,
			"outputKey":    "precursor",
			"stageIndex":   1.0,
			"scale":        0.0,
		}, "precursor").
		Poke(map[string]any{
			"input":     "spread",
			"outputKey": "compression",
			"scale":     0.0,
			"source":    "compression",
			"terms":     []string{"compression"},
		}, "compression").
		Poke(map[string]any{
			"terms":   []string{"rvol", "precursor"},
			"source":  "ignition",
			"combine": "ratio",
			"scale":   0.0,
		}, "ignition").
		Poke(map[string]any{
			"terms":   []string{"precursor", "compression", "rvol"},
			"inverts": []string{"compression"},
		}, "trend").
		Poke(map[string]any{
			"terms":   []string{"rvol", "precursor"},
			"inverts": []string{"rvol", "precursor"},
			"gate":    "rvolDecline",
		}, "exhaustion").
		Poke(map[string]any{
			"source":    "rvolDecline",
			"output":    "exhaustion",
			"squash":    0.0,
			"attenuate": []string{"compression"},
		}, "decline").
		Poke(map[string]any{
			"inputs": []string{"bid", "ask"},
		}, "spread").
		Poke(map[string]any{
			"leftKey":        "rvol",
			"rightKey":       "precursor",
			"destinationKey": "ignition",
			"source":         "ignition",
			"output":         "ignition",
		}, "joint")
}

func TestIgnitionSpreadAfterLogReturn(testingTB *testing.T) {
	Convey("Given features after log-return z-score in ignition", testingTB, func() {
		config := ignitionReplayConfig()
		stage := transport.NewPipeline(
			statistic.NewMeanMedianRatio(config),
			NewLogReturnZScore(config),
			vector.NewSpreadSample(config),
		)

		for _, tick := range ignitionWarmupTicks() {
			frame := datura.Acquire("ignition-spread-pipeline-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
			frame.Merge("features", []float64{tick.volume, tick.last, tick.last - 1, tick.last + 1})

			_ = transport.NewFlipFlop(frame, stage)
			frame.Release()
		}

		frame := datura.Acquire("ignition-spread-pipeline-frame", datura.APPJSON)
		frame.Merge("root", "features")
		frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		frame.Merge("features", []float64{120, 10050, 10050.0001, 10050.0002})

		err := transport.NewFlipFlop(frame, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "spread"), ShouldBeGreaterThan, 0)
		frame.Release()
	})
}

func ignitionWarmupTicks() []struct {
	volume float64
	last   float64
} {
	ticks := make([]struct {
		volume float64
		last   float64
	}, 0, 24)

	for index := range 24 {
		ticks = append(ticks, struct {
			volume float64
			last   float64
		}{
			volume: 1000 + float64(index)*10,
			last:   10000 + float64(index)*100,
		})
	}

	return ticks
}

func ignitionSpikeTick() (volume float64, last float64) {
	return 5000, 20000
}

func TestIgnitionSpreadOutput(testingTB *testing.T) {
	Convey("Given bid and ask features through ignition", testingTB, func() {
		config := ignitionReplayConfig()
		stage := NewIgnition(config)
		var frame *datura.Artifact

		for _, tick := range ignitionWarmupTicks() {
			next := datura.Acquire("ignition-spread-frame", datura.APPJSON)
			next.Merge("root", "features")
			next.Merge("inputs", []string{"volume", "last", "bid", "ask"})
			next.Merge("features", []float64{tick.volume, tick.last, tick.last - 1, tick.last + 1})

			_ = transport.NewFlipFlop(next, stage)

			if frame != nil {
				frame.Release()
			}

			frame = next
		}

		spikeVolume, spikeLast := ignitionSpikeTick()
		spikeFrame := datura.Acquire("ignition-spread-spike-frame", datura.APPJSON)
		spikeFrame.Merge("root", "features")
		spikeFrame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		spikeFrame.Merge("features", []float64{spikeVolume, spikeLast, spikeLast - 1, spikeLast + 1})

		err := transport.NewFlipFlop(spikeFrame, stage)

		if frame != nil {
			frame.Release()
		}

		defer spikeFrame.Release()

		So(err, ShouldBeNil)
		So(datura.Peek[float64](spikeFrame, "output", "spread"), ShouldBeGreaterThan, 0)
	})
}

func TestIgnitionReplayTraversal(testingTB *testing.T) {
	Convey("Given a long replay on one shared ignition config", testingTB, func() {
		config := ignitionReplayConfig()
		stage := NewIgnition(config)
		var artifact *datura.Artifact

		for _, tick := range ignitionWarmupTicks() {
			frame := datura.Acquire("ignition-replay-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
			frame.Merge("features", []float64{tick.volume, tick.last, tick.last - 1, tick.last + 1})

			_ = transport.NewFlipFlop(frame, stage)

			if artifact != nil {
				artifact.Release()
			}

			artifact = frame
		}

		spikeVolume, spikeLast := ignitionSpikeTick()
		spikeFrame := datura.Acquire("ignition-replay-spike-frame", datura.APPJSON)
		spikeFrame.Merge("root", "features")
		spikeFrame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		spikeFrame.Merge("features", []float64{spikeVolume, spikeLast, spikeLast - 1, spikeLast + 1})

		err := transport.NewFlipFlop(spikeFrame, stage)

		if artifact != nil {
			artifact.Release()
		}

		defer spikeFrame.Release()

		So(err, ShouldBeNil)

		Convey("It should still publish ignition logits after replay", func() {
			So(datura.Peek[float64](spikeFrame, "output", "rvol"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](spikeFrame, "output", "precursor"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](spikeFrame, "output", "ignition"), ShouldBeGreaterThanOrEqualTo, 0)
		})
	})
}

func BenchmarkIgnitionReplayTraversal(b *testing.B) {
	config := ignitionReplayConfig()
	stage := NewIgnition(config)

	for _, tick := range ignitionWarmupTicks() {
		frame := datura.Acquire("ignition-replay-warmup-frame", datura.APPJSON)
		frame.Merge("root", "features")
		frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		frame.Merge("features", []float64{tick.volume, tick.last, tick.last - 1, tick.last + 1})

		_ = transport.NewFlipFlop(frame, stage)
		frame.Release()
	}

	spikeVolume, _ := ignitionSpikeTick()
	baselineLast := 10000 + float64(len(ignitionWarmupTicks())-1)*100
	tickOffset := 0

	b.ReportAllocs()

	for b.Loop() {
		baselineVolume := 1000 + float64(tickOffset)*10
		baselineLast += 100

		baselineFrame := datura.Acquire("ignition-replay-baseline-frame", datura.APPJSON)
		baselineFrame.Merge("root", "features")
		baselineFrame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		baselineFrame.Merge("features", []float64{
			baselineVolume, baselineLast, baselineLast - 1, baselineLast + 1,
		})

		_ = transport.NewFlipFlop(baselineFrame, stage)
		baselineFrame.Release()

		spikeLastFrame := baselineLast * 2
		spikeVolumeFrame := spikeVolume + float64(tickOffset)
		tickOffset++

		frame := datura.Acquire("ignition-replay-bench-frame", datura.APPJSON)
		frame.Merge("root", "features")
		frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		frame.Merge("features", []float64{
			spikeVolumeFrame, spikeLastFrame, spikeLastFrame - 1, spikeLastFrame + 1,
		})

		if err := transport.NewFlipFlop(frame, stage); err != nil {
			b.Fatal(err)
		}

		frame.Release()
	}
}
