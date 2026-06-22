package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
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
			"inputs": []string{"bid", "ask"},
		}, "spread")
}

func TestSpreadSampleAfterMeanMedianRatio(testingTB *testing.T) {
	Convey("Given features after mean-median-ratio", testingTB, func() {
		config := ignitionSpreadConfig()
		stage := transport.NewPipeline(
			statistic.NewMeanMedianRatio(config),
			NewSpreadSample(config),
		)
		frame := datura.Acquire("spread-pipeline-frame", datura.APPJSON)
		frame.Merge("root", "features")
		frame.Merge("inputs", []string{"volume", "last", "bid", "ask"})
		frame.Merge("features", []float64{120, 10050, 10050.0001, 10050.0002})

		err := transport.NewFlipFlop(frame, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "spread"), ShouldBeGreaterThan, 0)
	})
}

func TestSpreadSampleRead(testingTB *testing.T) {
	Convey("Given bid and ask feature columns", testingTB, func() {
		config := datura.Acquire("spread-sample-config", datura.APPJSON).
			Poke([]string{"bid", "ask"}, "spread", "inputs")

		stage := NewSpreadSample(config)
		frame := datura.Acquire("spread-sample-frame", datura.APPJSON)
		frame.Merge("root", "features")
		frame.Merge("inputs", []string{"bid", "ask", "last"})
		frame.Merge("features", []float64{10050.0001, 10050.0002, 10050})

		err := transport.NewFlipFlop(frame, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "spread"), ShouldBeGreaterThan, 0)
	})
}
