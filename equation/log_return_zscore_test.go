package equation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestLogReturnZScoreRead(t *testing.T) {
	Convey("Given the composed precursor pipeline", t, func() {
		config := datura.Acquire("log-return-zscore-config", datura.APPJSON).
			Poke([]string{"rvol", "precursor"}, "order").
			Poke(map[string]any{
				"precursor": map[string]any{
					"input":        "last",
					"returnLag":    1.0,
					"longWindow":   5.0,
					"positiveOnly": 1.0,
					"outputKey":    "precursor",
					"stageIndex":   1.0,
				},
			}, "inputs")

		stage := NewLogReturnZScore(config)
		var lastFrame *datura.Artifact

		for _, last := range []float64{100, 101, 102, 103, 104, 200} {
			frame := datura.Acquire("log-return-zscore-frame", datura.APPJSON)
			frame.Merge("root", "features")
			frame.Merge("inputs", []string{"volume", "last"})
			frame.Merge("features", []float64{1000, last})

			err := transport.NewFlipFlop(frame, stage)

			So(err, ShouldBeNil)

			if lastFrame != nil {
				lastFrame.Release()
			}

			lastFrame = frame
		}

		defer lastFrame.Release()

		Convey("It should publish a non-negative precursor score", func() {
			So(datura.Peek[float64](lastFrame, "output", "precursor"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkLogReturnZScoreRead(b *testing.B) {
	config := datura.Acquire("log-return-zscore-bench", datura.APPJSON).
		Poke([]string{"rvol", "precursor"}, "order").
		Poke(map[string]any{
			"precursor": map[string]any{
				"input":        "last",
				"returnLag":    1.0,
				"longWindow":   5.0,
				"positiveOnly": 1.0,
				"outputKey":    "precursor",
			},
		}, "inputs")

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
