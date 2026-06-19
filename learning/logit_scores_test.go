package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestLogitScoresRead(testingTB *testing.T) {
	Convey("Given order, inputs, and outputs on the config artifact", testingTB, func() {
		config := datura.Acquire("logit-scores-config", datura.APPJSON).
			Poke([]string{"rvol", "precursor", "compression"}, "order").
			Poke([]string{"ignition", "compression", "trend", "exhaustion"}, "outputs").
			Poke(2.0, "threshold").
			Poke(map[string]any{
				"rvol": map[string]any{
					"source": "rvol",
					"scale":  2.5,
				},
				"precursor": map[string]any{
					"source": "precursor",
					"scale":  2.0,
				},
				"compression": map[string]any{
					"source": "value",
					"scale":  1.5,
				},
				"joint": map[string]any{
					"source": "ignition",
					"output": "ignition",
				},
			}, "inputs")

		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-test", datura.APPJSON)
		artifact.Poke(3.0, "output", "rvol")
		artifact.Poke(2.5, "output", "precursor")
		artifact.Poke(0.8, "output", "value")
		artifact.Poke(2.7, "output", "ignition")

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should publish configured classifier logits", func() {
			So(datura.Peek[float64](artifact, "output", "ignition"), ShouldEqual, 2.7)
			So(datura.Peek[float64](artifact, "output", "compression"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "trend"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "exhaustion"), ShouldBeGreaterThan, 0)
		})
	})
}
