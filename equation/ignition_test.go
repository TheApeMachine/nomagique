package equation

import (
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/vector"
)

func ignitionLogitConfig() *datura.Artifact {
	return datura.Acquire("pumpdump-ignition-replay", datura.APPJSON).
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
			"scaleMode":   "median",
			"centerMode":  "median",
			"leftKey":     "rvol",
			"rightKey":    "precursor",
			"decline":     map[string]any{"output": "rvolDecline"},
		}, "rvol").
		Poke(map[string]any{
			"input":        "last",
			"returnLag":    0.0,
			"longWindow":   0.0,
			"positiveOnly": 1.0,
			"outputKey":    "precursor",
			"stageIndex":   1.0,
			"scale":        0.0,
			"scaleMode":    "median",
			"leftKey":      "rvol",
			"rightKey":     "precursor",
		}, "precursor").
		Poke(map[string]any{
			"inputs": []string{"bid", "ask"},
		}, "spread").
		Poke(map[string]any{
			"terms":     []string{"rvol", "precursor"},
			"source":    "ignition",
			"combine":   "ratio",
			"scale":     0.0,
			"leftKey":   "rvol",
			"rightKey":  "precursor",
			"scaleMode": "median",
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
			"input":      "spread",
			"outputKey":  "compression",
			"scale":      0.0,
			"scaleMode":  "median",
			"terms":      []string{"compression", "precursor", "rvol"},
			"inverts":    []string{"precursor", "rvol"},
			"gate":       "precursor",
			"gateInvert": 1.0,
			"leftKey":    "rvol",
			"rightKey":   "precursor",
		}, "compression").
		Poke(map[string]any{
			"source":    "rvolDecline",
			"output":    "exhaustion",
			"squash":    0.0,
			"attenuate": []string{"compression"},
		}, "decline")
}

func TestIgnitionFeatureExtractorPipeline(testingTB *testing.T) {
	Convey("Given FeatureExtractor through Ignition and Classifier", testingTB, func() {
		schema := datura.Acquire("ignition-pipeline-schema", datura.APPJSON).WithAttributes(datura.Map[any]{
			"required": []string{"ticker"},
			"ticker": datura.Map[any]{
				"root":         "data",
				"elementIndex": 0.0,
				"inputs":       []string{"symbol", "bid", "ask", "last", "volume"},
			},
		})

		pipeline := nomagique.Number(
			vector.NewFeatureExtractor(schema),
			NewIgnition(ignitionLogitConfig()),
			probability.NewClassifier(
				datura.Acquire("ignition-pipeline-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
					"inputs": []string{"ignition", "compression", "trend", "exhaustion"},
				}),
			),
		)

		var lastFrame *datura.Artifact
		timestamp := time.Unix(0, 1).UnixNano()

		for _, tick := range ignitionWarmupTicks() {
			payload := fmt.Sprintf(
				`{"channel":"ticker","type":"update","data":[{"symbol":"BTC/USD","bid":%g,"ask":%g,"last":%g,"volume":%g}]}`,
				tick.last-1, tick.last+1, tick.last, tick.volume,
			)
			frame := datura.Acquire("ignition-pipeline-frame", datura.APPJSON)
			frame.WithPayload([]byte(payload))
			frame.SetTimestamp(timestamp)
			timestamp += int64(time.Second)

			_ = transport.NewFlipFlop(frame, pipeline)

			if lastFrame != nil {
				lastFrame.Release()
			}

			lastFrame = frame
		}

		spikeVolume, spikeLast := ignitionSpikeTick()
		spikePayload := fmt.Sprintf(
			`{"channel":"ticker","type":"update","data":[{"symbol":"BTC/USD","bid":%g,"ask":%g,"last":%g,"volume":%g}]}`,
			spikeLast-1, spikeLast+1, spikeLast, spikeVolume,
		)
		spikeFrame := datura.Acquire("ignition-pipeline-spike", datura.APPJSON)
		spikeFrame.WithPayload([]byte(spikePayload))
		spikeFrame.SetTimestamp(timestamp)

		err := transport.NewFlipFlop(spikeFrame, pipeline)

		if lastFrame != nil {
			lastFrame.Release()
		}

		defer spikeFrame.Release()

		Convey("It should classify ignition from the full ticker pipeline", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[float64](spikeFrame, "output", "rvol"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](spikeFrame, "output", "precursor"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](spikeFrame, "output", "category")), ShouldBeBetweenOrEqual, 1, 4)
			So(datura.Peek[float64](spikeFrame, "output", "confidence"), ShouldBeGreaterThan, 0.25)
		})
	})
}

func TestIgnitionSpreadAfterLogReturn(testingTB *testing.T) {
	Convey("Given features through ignition", testingTB, func() {
		stage := NewIgnition(ignitionLogitConfig())
		timestamp := time.Unix(0, 1).UnixNano()

		for _, tick := range ignitionWarmupTicks() {
			frame := datura.Acquire("ignition-spread-pipeline-frame", datura.APPJSON)
			frame.Poke("features", "root")
			frame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
			frame.Merge("features", []float64{tick.volume, tick.last, tick.last - 1, tick.last + 1})
			frame.SetTimestamp(timestamp)
			timestamp += int64(time.Second)

			_ = transport.NewFlipFlop(frame, stage)
			frame.Release()
		}

		frame := datura.Acquire("ignition-spread-pipeline-frame", datura.APPJSON)
		frame.Poke("features", "root")
		frame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
		frame.Merge("features", []float64{120, 10050, 10050.0001, 10050.0002})
		frame.SetTimestamp(timestamp)

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
		config := ignitionLogitConfig()
		stage := NewIgnition(config)
		var frame *datura.Artifact
		timestamp := time.Unix(0, 1).UnixNano()

		for _, tick := range ignitionWarmupTicks() {
			next := datura.Acquire("ignition-spread-frame", datura.APPJSON)
			next.Poke("features", "root")
			next.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
			next.Merge("features", []float64{tick.volume, tick.last, tick.last - 1, tick.last + 1})
			next.SetTimestamp(timestamp)
			timestamp += int64(time.Second)

			_ = transport.NewFlipFlop(next, stage)

			if frame != nil {
				frame.Release()
			}

			frame = next
		}

		spikeVolume, spikeLast := ignitionSpikeTick()
		spikeFrame := datura.Acquire("ignition-spread-spike-frame", datura.APPJSON)
		spikeFrame.Poke("features", "root")
		spikeFrame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
		spikeFrame.Merge("features", []float64{spikeVolume, spikeLast, spikeLast - 1, spikeLast + 1})
		spikeFrame.SetTimestamp(timestamp)

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
		config := ignitionLogitConfig()
		stage := NewIgnition(config)
		var artifact *datura.Artifact
		timestamp := time.Unix(0, 1).UnixNano()

		for _, tick := range ignitionWarmupTicks() {
			frame := datura.Acquire("ignition-replay-frame", datura.APPJSON)
			frame.Poke("features", "root")
			frame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
			frame.Merge("features", []float64{tick.volume, tick.last, tick.last - 1, tick.last + 1})
			frame.SetTimestamp(timestamp)
			timestamp += int64(time.Second)

			_ = transport.NewFlipFlop(frame, stage)

			if artifact != nil {
				artifact.Release()
			}

			artifact = frame
		}

		spikeVolume, spikeLast := ignitionSpikeTick()
		spikeFrame := datura.Acquire("ignition-replay-spike-frame", datura.APPJSON)
		spikeFrame.Poke("features", "root")
		spikeFrame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
		spikeFrame.Merge("features", []float64{spikeVolume, spikeLast, spikeLast - 1, spikeLast + 1})
		spikeFrame.SetTimestamp(timestamp)

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
	config := ignitionLogitConfig()
	stage := NewIgnition(config)

	for _, tick := range ignitionWarmupTicks() {
		frame := datura.Acquire("ignition-replay-warmup-frame", datura.APPJSON)
		frame.Poke("features", "root")
		frame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
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
		baselineFrame.Poke("features", "root")
		baselineFrame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
		baselineFrame.Merge("features", []float64{
			baselineVolume, baselineLast, baselineLast - 1, baselineLast + 1,
		})

		_ = transport.NewFlipFlop(baselineFrame, stage)
		baselineFrame.Release()

		spikeLastFrame := baselineLast * 2
		spikeVolumeFrame := spikeVolume + float64(tickOffset)
		tickOffset++

		frame := datura.Acquire("ignition-replay-bench-frame", datura.APPJSON)
		frame.Poke("features", "root")
		frame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
		frame.Merge("features", []float64{
			spikeVolumeFrame, spikeLastFrame, spikeLastFrame - 1, spikeLastFrame + 1,
		})

		if err := transport.NewFlipFlop(frame, stage); err != nil {
			b.Fatal(err)
		}

		frame.Release()
	}
}
