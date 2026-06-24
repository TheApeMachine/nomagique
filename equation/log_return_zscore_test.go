package equation

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestLogReturnZScoreRead(t *testing.T) {
	Convey("Given the composed precursor pipeline", t, func() {
		config := datura.Acquire("log-return-zscore-config", datura.APPJSON).
			Poke("precursor", "stage").
			Poke(map[string]any{
				"input":        "last",
				"returnLag":    1.0,
				"longWindow":   5.0,
				"positiveOnly": 1.0,
				"outputKey":    "precursor",
				"seriesKey":    "precursor",
			}, "precursor")

		stage := NewLogReturnZScore(config)
		var lastFrame *datura.Artifact
		timestamp := time.Unix(0, 1).UnixNano()

		for _, last := range []float64{100, 101, 102, 103, 104, 200} {
			frame := datura.Acquire("log-return-zscore-frame", datura.APPJSON)
			frame.Poke("features", "root")
			frame.Poke([]string{"volume", "last"}, "inputs")
			frame.Merge("features", []float64{1000, last})
			frame.SetTimestamp(timestamp)
			timestamp += int64(time.Second)

			err := transport.NewFlipFlop(frame, stage)

			if err != nil {
				frame.Release()

				continue
			}

			if lastFrame != nil {
				lastFrame.Release()
			}

			lastFrame = frame
		}

		So(lastFrame, ShouldNotBeNil)

		defer lastFrame.Release()

		Convey("It should publish a non-negative precursor score", func() {
			So(datura.Peek[float64](lastFrame, "output", "precursor"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkLogReturnZScoreRead(b *testing.B) {
	config := datura.Acquire("log-return-zscore-bench", datura.APPJSON).
		Poke("precursor", "stage").
		Poke(map[string]any{
			"input":        "last",
			"returnLag":    1.0,
			"longWindow":   5.0,
			"positiveOnly": 1.0,
			"outputKey":    "precursor",
		}, "precursor")

	stage := NewLogReturnZScore(config)
	artifact := datura.Acquire("log-return-zscore-bench-test", datura.APPJSON).
		Poke("features", "root").
		Poke([]string{"volume", "last"}, "inputs").
		Poke([]float64{1000, 105}, "features")

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
