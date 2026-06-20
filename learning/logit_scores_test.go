package learning

import (
	"math"
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
			So(datura.Peek[float64](artifact, "output", "ignition"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "compression"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "trend"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "exhaustion"), ShouldEqual, 0)
		})
	})

	Convey("Given zero precursor with dynamic feature scales", testingTB, func() {
		config := datura.Acquire("logit-scores-zero-precursor", datura.APPJSON).
			Poke([]string{"rvol", "precursor", "compression"}, "order").
			Poke([]string{"ignition", "compression", "trend", "exhaustion"}, "outputs").
			Poke(0.0, "threshold").
			Poke(map[string]any{
				"rvol": map[string]any{
					"source": "rvol",
					"scale":  0.0,
				},
				"precursor": map[string]any{
					"source": "precursor",
					"scale":  0.0,
				},
				"compression": map[string]any{
					"source": "value",
					"scale":  0.0,
				},
				"joint": map[string]any{
					"source": "ignition",
					"output": "ignition",
				},
			}, "inputs")

		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-zero-precursor-test", datura.APPJSON)
		artifact.Poke(9.125, "output", "rvol")
		artifact.Poke(0.0, "output", "precursor")
		artifact.Poke(1.0, "output", "value")
		artifact.Poke(0.0, "output", "ignition")

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should publish finite logits without NaN weights", func() {
			ignition := datura.Peek[float64](artifact, "output", "ignition")
			compression := datura.Peek[float64](artifact, "output", "compression")
			trend := datura.Peek[float64](artifact, "output", "trend")
			exhaustion := datura.Peek[float64](artifact, "output", "exhaustion")

			So(math.IsNaN(ignition), ShouldBeFalse)
			So(math.IsNaN(compression), ShouldBeFalse)
			So(math.IsNaN(trend), ShouldBeFalse)
			So(math.IsNaN(exhaustion), ShouldBeFalse)
			So(compression, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given fading volume lift with flat precursor", testingTB, func() {
		config := datura.Acquire("logit-scores-decline", datura.APPJSON).
			Poke([]string{"rvol", "precursor", "compression"}, "order").
			Poke([]string{"ignition", "compression", "trend", "exhaustion"}, "outputs").
			Poke(2.0, "threshold").
			Poke(0.95, "state", "rvolDecline").
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
		artifact := datura.Acquire("logit-scores-decline-test", datura.APPJSON)
		artifact.Poke(1.2, "output", "rvol")
		artifact.Poke(0.05, "output", "precursor")
		artifact.Poke(0.5, "output", "value")
		artifact.Poke(0.1, "output", "ignition")

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should favor exhaustion over coiled compression", func() {
			coiled := datura.Peek[float64](artifact, "output", "compression")
			exhaustion := datura.Peek[float64](artifact, "output", "exhaustion")

			So(exhaustion, ShouldBeGreaterThan, coiled)
		})
	})
}

func BenchmarkLogitScoresRead(testingTB *testing.B) {
	config := datura.Acquire("logit-scores-bench", datura.APPJSON).
		Poke([]string{"rvol", "precursor", "compression"}, "order").
		Poke([]string{"ignition", "compression", "trend", "exhaustion"}, "outputs").
		Poke(2.0, "threshold").
		Poke(0.5, "state", "rvolDecline").
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
	artifact := datura.Acquire("logit-scores-bench-test", datura.APPJSON)
	artifact.Poke(1.2, "output", "rvol")
	artifact.Poke(0.05, "output", "precursor")
	artifact.Poke(0.5, "output", "value")
	artifact.Poke(0.1, "output", "ignition")

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
