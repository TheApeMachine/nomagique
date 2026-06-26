package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/statistic"
)

func ignitionSpreadConfig() *datura.Artifact {
	return datura.Acquire("spread-pipeline-config", datura.APPJSON).
		Poke(0.0, "stageIndex").
		Poke([]string{"rvol", "precursor", "compression"}, "order").
		Poke(map[string]any{
			"input":       "volume",
			"transform":   "deltaPositive",
			"shortWindow": 0.0,
			"longWindow":  0.0,
			"outputKey":   "rvol",
		}, "rvol").
		Poke(map[string]any{
			"inputs":    []string{"bid", "ask"},
			"outputKey": "spread",
		}, "spread")
}

func TestSpreadSampleAfterMeanMedianRatio(testingTB *testing.T) {
	Convey("Given features after mean-median-ratio", testingTB, func() {
		config := ignitionSpreadConfig()
		stage := transport.NewPipeline(
			statistic.NewMeanMedianRatio(config),
			NewSpreadSample(config),
		)

		for index := range 24 {
			frame := datura.Acquire("spread-pipeline-warmup-frame", datura.APPJSON)
			last := 10000 + float64(index)*100
			frame.Poke("features", "root")
			frame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
			frame.Merge("features", []float64{1000 + float64(index)*10, last, last - 1, last + 1})

			_ = nomagique.RoundTripArtifact(frame, stage)
			frame.Release()
		}

		frame := datura.Acquire("spread-pipeline-frame", datura.APPJSON)
		frame.Poke("features", "root")
		frame.Poke([]string{"volume", "last", "bid", "ask"}, "inputs")
		frame.Merge("features", []float64{120, 10050, 10050.0001, 10050.0002})

		err := nomagique.RoundTripArtifact(frame, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "spread"), ShouldBeGreaterThan, 0)
	})
}

func TestSpreadSampleRead(testingTB *testing.T) {
	Convey("Given bid and ask feature columns", testingTB, func() {
		config := datura.Acquire("spread-sample-config", datura.APPJSON).
			Poke([]string{"bid", "ask"}, "spread", "inputs").
			Poke("spread", "spread", "outputKey")

		stage := NewSpreadSample(config)
		frame := datura.Acquire("spread-sample-frame", datura.APPJSON)
		frame.Poke("features", "root")
		frame.Poke([]string{"bid", "ask", "last"}, "inputs")
		frame.Merge("features", []float64{10050.0001, 10050.0002, 10050})

		err := nomagique.RoundTripArtifact(frame, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "spread"), ShouldBeGreaterThan, 0)
	})
}
