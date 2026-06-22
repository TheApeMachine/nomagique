package learning

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func logitScoresConfig() *datura.Artifact {
	return datura.Acquire("logit-scores-config", datura.APPJSON).WithAttributes(datura.Map[any]{
		"root": "output",
		"inputs": []string{
			"rvol", "precursor", "compression", "spread", "ignition", "value", "rvolDecline",
		},
		"order":     []string{"rvol", "precursor", "compression"},
		"outputs":   []string{"ignition", "compression", "trend", "exhaustion"},
		"threshold": 2.0,
		"rvol": datura.Map[any]{
			"source": "rvol",
			"scale":  2.5,
		},
		"precursor": datura.Map[any]{
			"source": "precursor",
			"scale":  2.0,
		},
		"compression": datura.Map[any]{
			"source": "value",
			"scale":  1.5,
			"terms":  []string{"compression", "precursor"},
			"inverts": []string{
				"precursor",
			},
		},
		"ignition": datura.Map[any]{
			"terms":   []string{"rvol", "precursor"},
			"source":  "ignition",
			"combine": "ratio",
		},
		"trend": datura.Map[any]{
			"terms":   []string{"precursor", "compression", "rvol"},
			"inverts": []string{"compression"},
		},
		"exhaustion": datura.Map[any]{
			"terms":   []string{"rvol", "precursor"},
			"inverts": []string{"rvol", "precursor"},
			"gate":    "rvolDecline",
		},
		"decline": datura.Map[any]{
			"source":    "rvolDecline",
			"output":    "exhaustion",
			"squash":    0.0,
			"attenuate": []string{"compression"},
		},
		"joint": datura.Map[any]{
			"source":    "ignition",
			"output":    "ignition",
			"combine":   "ratio",
			"scaleMode": "median",
		},
	})
}

func TestLogitScoresRead(testingTB *testing.T) {
	Convey("Given order, inputs, and outputs on the config artifact", testingTB, func() {
		config := logitScoresConfig()
		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-test", datura.APPJSON)
		artifact.Merge("root", "output")
		artifact.Merge("inputs", []string{"rvol", "precursor", "compression", "ignition", "value"})
		artifact.MergeOutput("rvol", 3.0)
		artifact.MergeOutput("precursor", 2.5)
		artifact.MergeOutput("value", 0.8)
		artifact.MergeOutput("ignition", 2.7)
		artifact.MergeOutput("rvolDecline", 0.0)

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should publish configured classifier logits", func() {
			So(datura.Peek[float64](artifact, "output", "ignition"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "compression"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "trend"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "exhaustion"), ShouldBeGreaterThanOrEqualTo, 0)
		})
	})

	Convey("Given zero precursor with dynamic feature scales", testingTB, func() {
		config := logitScoresConfig()
		config.Poke(0.0, "compression", "scale")
		config.Poke(map[string]any{
			"terms": []string{"rvol", "precursor"},
		}, "ignition")

		stage := NewLogitScores(config)
		var lastArtifact *datura.Artifact

		for _, sample := range []struct {
			rvol, precursor, value float64
		}{
			{9.0, 0.5, 0.8},
			{9.125, 0.0, 1.0},
		} {
			artifact := datura.Acquire("logit-scores-zero-precursor-test", datura.APPJSON)
			artifact.Merge("root", "output")
			artifact.Merge("inputs", []string{"rvol", "precursor", "compression", "ignition", "value"})
			artifact.MergeOutput("rvol", sample.rvol)
			artifact.MergeOutput("precursor", sample.precursor)
			artifact.MergeOutput("value", sample.value)
			artifact.MergeOutput("ignition", 0.0)
			artifact.MergeOutput("rvolDecline", 0.0)

			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		defer lastArtifact.Release()

		Convey("It should publish finite logits without NaN weights", func() {
			ignition := datura.Peek[float64](lastArtifact, "output", "ignition")
			compression := datura.Peek[float64](lastArtifact, "output", "compression")
			trend := datura.Peek[float64](lastArtifact, "output", "trend")
			exhaustion := datura.Peek[float64](lastArtifact, "output", "exhaustion")

			So(math.IsNaN(ignition), ShouldBeFalse)
			So(math.IsNaN(compression), ShouldBeFalse)
			So(math.IsNaN(trend), ShouldBeFalse)
			So(math.IsNaN(exhaustion), ShouldBeFalse)
			So(compression, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given fading volume lift with flat precursor", testingTB, func() {
		config := logitScoresConfig()

		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-decline-test", datura.APPJSON)
		artifact.Merge("root", "output")
		artifact.Merge("inputs", []string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"})
		artifact.MergeOutput("rvol", 1.2)
		artifact.MergeOutput("precursor", 0.05)
		artifact.MergeOutput("value", 0.5)
		artifact.MergeOutput("ignition", 0.1)
		artifact.MergeOutput("rvolDecline", 0.95)

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
	config := logitScoresConfig()

	stage := NewLogitScores(config)
	artifact := datura.Acquire("logit-scores-bench-test", datura.APPJSON)
	artifact.Merge("root", "output")
	artifact.Merge("inputs", []string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"})
	artifact.MergeOutput("rvol", 1.2)
	artifact.MergeOutput("precursor", 0.05)
	artifact.MergeOutput("value", 0.5)
	artifact.MergeOutput("ignition", 0.1)
	artifact.MergeOutput("rvolDecline", 0.5)

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
