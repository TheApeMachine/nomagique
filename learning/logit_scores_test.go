package learning

import (
	"io"
	"math"
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
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
			"terms":    []string{"rvol", "precursor"},
			"source":   "ignition",
			"combine":  "ratio",
			"leftKey":  "rvol",
			"rightKey": "precursor",
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
	Convey("Given nested output config from WithAttributes", testingTB, func() {
		config := datura.Acquire("logit-scores-nested-config", datura.APPJSON).WithAttributes(datura.Map[any]{
			"ignition": datura.Map[any]{
				"leftKey":  "rvol",
				"rightKey": "precursor",
			},
		})

		config.Poke([]float64{1}, "output", "scaleSamples", "rvol")

		Convey("It should keep nested operand keys readable after runtime config state writes", func() {
			So(datura.Peek[string](config, "ignition", "leftKey"), ShouldEqual, "rvol")
			So(datura.Peek[string](config, "ignition", "rightKey"), ShouldEqual, "precursor")
		})
	})

	Convey("Given no inbound frame", testingTB, func() {
		stage := NewLogitScores(logitScoresConfig())
		buffer := make([]byte, 4096)

		n, err := stage.Read(buffer)

		Convey("It should report no frame without trying to unpack stale payload", func() {
			So(n, ShouldEqual, 0)
			So(err, ShouldEqual, io.EOF)
		})
	})

	Convey("Given an empty inbound frame", testingTB, func() {
		stage := NewLogitScores(logitScoresConfig())
		buffer := make([]byte, 4096)

		n, writeErr := stage.Write(nil)
		readN, readErr := stage.Read(buffer)

		Convey("It should drain cleanly without validation noise", func() {
			So(n, ShouldEqual, 0)
			So(writeErr, ShouldBeNil)
			So(readN, ShouldEqual, 0)
			So(readErr, ShouldEqual, io.EOF)
		})
	})

	Convey("Given order, inputs, and outputs on the config artifact", testingTB, func() {
		config := logitScoresConfig()
		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-test", datura.APPJSON)
		artifact.Poke("output", "root")
		artifact.Poke([]string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"}, "inputs")
		artifact.MergeOutput("rvol", 3.0)
		artifact.MergeOutput("precursor", 2.5)
		artifact.MergeOutput("value", 0.8)
		artifact.MergeOutput("ignition", 2.7)
		artifact.MergeOutput("rvolDecline", 0.0)

		err := nomagique.RoundTripArtifact(artifact, stage)

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
			artifact.Poke("output", "root")
			artifact.Poke([]string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"}, "inputs")
			artifact.MergeOutput("rvol", sample.rvol)
			artifact.MergeOutput("precursor", sample.precursor)
			artifact.MergeOutput("value", sample.value)
			artifact.MergeOutput("ignition", 0.0)
			artifact.MergeOutput("rvolDecline", 0.0)

			err := nomagique.RoundTripArtifact(artifact, stage)

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

	Convey("Given spread-sourced threshold and composite wire scales", testingTB, func() {
		config := logitScoresConfig()
		config.Poke(0.0, "threshold")
		config.Poke(map[string]any{
			"source": "spread",
		}, "threshold")
		config.Poke(0.0, "rvol", "scale")
		config.Poke(0.0, "precursor", "scale")
		config.Poke(1.0, "compression", "scale")
		config.Poke("median", "rvol", "scaleMode")
		config.Poke("median", "precursor", "scaleMode")
		config.Poke("static", "compression", "scaleMode")
		config.Poke("spread", "rvol", "leftKey")
		config.Poke("spread", "rvol", "rightKey")
		config.Poke("spread", "precursor", "leftKey")
		config.Poke("spread", "precursor", "rightKey")
		config.Poke("spread", "compression", "leftKey")
		config.Poke("spread", "compression", "rightKey")
		config.Poke(map[string]any{
			"terms": []string{"rvol", "precursor"},
		}, "ignition")

		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-spread-threshold-test", datura.APPJSON)
		artifact.Poke("output", "root")
		artifact.Poke([]string{"rvol", "precursor", "compression", "spread", "ignition", "value", "rvolDecline"}, "inputs")
		artifact.MergeOutput("rvol", 0.0001)
		artifact.MergeOutput("precursor", 0.0001)
		artifact.MergeOutput("value", 0.0)
		artifact.MergeOutput("spread", 0.0001)
		artifact.MergeOutput("compression", 0.5)
		artifact.MergeOutput("ignition", 0.0)
		artifact.MergeOutput("rvolDecline", 0.0)

		err := nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should publish finite logits from spread-derived threshold", func() {
			So(datura.Peek[float64](artifact, "output", "ignition"), ShouldBeGreaterThanOrEqualTo, 0)
			So(math.IsNaN(datura.Peek[float64](artifact, "output", "trend")), ShouldBeFalse)
		})

		artifact.Release()
	})

	Convey("Given first-frame median-scaled ratio output", testingTB, func() {
		config := logitScoresConfig()
		config.Poke(0.0, "rvol", "scale")
		config.Poke(0.0, "precursor", "scale")
		config.Poke("median", "rvol", "scaleMode")
		config.Poke("median", "precursor", "scaleMode")

		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-first-ratio-test", datura.APPJSON)
		artifact.Poke("output", "root")
		artifact.Poke([]string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"}, "inputs")
		artifact.MergeOutput("rvol", 1.0)
		artifact.MergeOutput("precursor", 1.2)
		artifact.MergeOutput("compression", 0.2)
		artifact.MergeOutput("value", 0.2)
		artifact.MergeOutput("ignition", math.Sqrt(1.0*1.2))
		artifact.MergeOutput("rvolDecline", 0.0)

		err := nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should use the current observation as the initial scale sample", func() {
			So(datura.Peek[float64](artifact, "output", "ignition"), ShouldBeGreaterThan, 0)
			So(math.IsNaN(datura.Peek[float64](artifact, "output", "ignition")), ShouldBeFalse)
		})

		artifact.Release()
	})

	Convey("Given fading volume lift with flat precursor", testingTB, func() {
		config := logitScoresConfig()

		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-decline-test", datura.APPJSON)
		artifact.Poke("output", "root")
		artifact.Poke([]string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"}, "inputs")
		artifact.MergeOutput("rvol", 1.2)
		artifact.MergeOutput("precursor", 0.05)
		artifact.MergeOutput("value", 0.5)
		artifact.MergeOutput("ignition", 0.1)
		artifact.MergeOutput("rvolDecline", 0.95)

		err := nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should favor exhaustion over coiled compression", func() {
			coiled := datura.Peek[float64](artifact, "output", "compression")
			exhaustion := datura.Peek[float64](artifact, "output", "exhaustion")

			So(exhaustion, ShouldBeGreaterThan, coiled)
		})
	})
	Convey("Given elevated precursor with gateInvert on compression", testingTB, func() {
		config := logitScoresConfig()
		config.Poke(map[string]any{
			"scale":      1.5,
			"terms":      []string{"compression", "precursor"},
			"inverts":    []string{"precursor"},
			"gate":       "precursor",
			"gateInvert": 1.0,
		}, "compression")

		stage := NewLogitScores(config)
		artifact := datura.Acquire("logit-scores-gate-invert-test", datura.APPJSON)
		artifact.Poke("output", "root")
		artifact.Poke([]string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"}, "inputs")
		artifact.MergeOutput("rvol", 3.0)
		artifact.MergeOutput("precursor", 2.5)
		artifact.MergeOutput("compression", 0.8)
		artifact.MergeOutput("ignition", 2.7)
		artifact.MergeOutput("rvolDecline", 0.0)

		err := nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should suppress compression when precursor is elevated", func() {
			So(datura.Peek[float64](artifact, "output", "compression"), ShouldAlmostEqual, 0, 0.0001)
			So(datura.Peek[float64](artifact, "output", "ignition"), ShouldBeGreaterThan, 0)
		})

		artifact.Release()
	})
	Convey("Given median-centered lift with elevated precursor", testingTB, func() {
		config := logitScoresConfig()
		config.Poke(0.0, "rvol", "scale")
		config.Poke(0.0, "precursor", "scale")
		config.Poke(1.0, "compression", "scale")
		config.Poke("median", "rvol", "scaleMode")
		config.Poke("median", "precursor", "scaleMode")
		config.Poke("static", "compression", "scaleMode")
		config.Poke("median", "rvol", "centerMode")

		stage := NewLogitScores(config)
		var artifact *datura.Artifact

		for _, sample := range []struct {
			rvol      float64
			precursor float64
			ignition  float64
		}{
			{1.0, 0.1, math.Sqrt(1.0 * 0.1)},
			{1.01, 10.0, math.Sqrt(1.01 * 10.0)},
		} {
			if artifact != nil {
				artifact.Release()
			}

			artifact = datura.Acquire("logit-scores-centered-test", datura.APPJSON)
			artifact.Poke("output", "root")
			artifact.Poke(
				[]string{
					"rvol", "precursor", "compression",
					"ignition", "value", "rvolDecline",
				},
				"inputs",
			)
			artifact.MergeOutput("rvol", sample.rvol)
			artifact.MergeOutput("precursor", sample.precursor)
			artifact.MergeOutput("compression", 0.0001)
			artifact.MergeOutput("value", 0.0001)
			artifact.MergeOutput("ignition", sample.ignition)
			artifact.MergeOutput("rvolDecline", 0.0)

			err := nomagique.RoundTripArtifact(artifact, stage)

			So(err, ShouldBeNil)
		}

		defer artifact.Release()

		Convey("It should not let normal lift turn precursor into ignition", func() {
			So(
				datura.Peek[float64](artifact, "output", "trend"),
				ShouldBeGreaterThan,
				datura.Peek[float64](artifact, "output", "ignition"),
			)
		})
	})
}

func TestLogitScoresReadClassifiedCategories(testingTB *testing.T) {
	Convey("Given controlled pumpdump ticker score inputs", testingTB, func() {
		type logitCase struct {
			name         string
			rvol         float64
			precursor    float64
			value        float64
			ignition     float64
			rvolDecline  float64
			wantCategory int
			wantScore    string
		}

		assertHighestProbability := func(outbound *datura.Artifact, category int) {
			probabilities := datura.Peek[[]float64](outbound, "output", "probabilities")
			distribution := datura.Peek[map[string]any](outbound, "output", "distribution")
			So(len(probabilities), ShouldEqual, 4)
			So(len(distribution), ShouldEqual, 4)

			selected := probabilities[category-1]
			total := 0.0

			for index, probability := range probabilities {
				total += probability
				mass, ok := distribution[strconv.Itoa(index+1)].(float64)
				So(ok, ShouldBeTrue)
				So(mass, ShouldAlmostEqual, probability, 1e-12)
				if index != category-1 {
					So(selected, ShouldBeGreaterThan, probability)
				}
			}

			So(total, ShouldAlmostEqual, 1.0, 1e-12)
		}

		cases := []logitCase{
			{
				name:         "vertical ignition",
				rvol:         3.0,
				precursor:    2.5,
				value:        0.1,
				ignition:     20.0,
				wantCategory: 2,
				wantScore:    "ignition",
			},
			{
				name:         "coiled compression",
				rvol:         0.1,
				precursor:    0.1,
				value:        20.0,
				ignition:     0.0,
				wantCategory: 1,
				wantScore:    "compression",
			},
			{
				name:         "organic trend",
				rvol:         3.0,
				precursor:    3.0,
				value:        0.0,
				ignition:     0.0,
				wantCategory: 3,
				wantScore:    "trend",
			},
			{
				name:         "faded exhaustion",
				rvol:         0.1,
				precursor:    0.1,
				value:        0.0,
				ignition:     0.0,
				rvolDecline:  1.0,
				wantCategory: 4,
				wantScore:    "exhaustion",
			},
		}

		for _, testCase := range cases {
			testCase := testCase

			Convey("When classifying "+testCase.name, func() {
				stage := nomagique.Number(
					NewLogitScores(logitScoresConfig()),
					probability.NewClassifier(
						datura.Acquire("logit-scores-classifier", datura.APPJSON).Poke(
							[]string{"compression", "ignition", "trend", "exhaustion"},
							"inputs",
						),
					),
				)
				artifact := datura.Acquire("logit-scores-classified-test", datura.APPJSON)
				artifact.Poke("output", "root")
				artifact.Poke([]string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"}, "inputs")
				artifact.MergeOutput("rvol", testCase.rvol)
				artifact.MergeOutput("precursor", testCase.precursor)
				artifact.MergeOutput("compression", testCase.value)
				artifact.MergeOutput("value", testCase.value)
				artifact.MergeOutput("ignition", testCase.ignition)
				artifact.MergeOutput("rvolDecline", testCase.rvolDecline)

				err := nomagique.RoundTripArtifact(artifact, stage)

				So(err, ShouldBeNil)

				Convey("It should put the intended ticker category on top", func() {
					So(int(datura.Peek[float64](artifact, "output", "category")), ShouldEqual, testCase.wantCategory)
					So(datura.Peek[float64](artifact, "output", testCase.wantScore), ShouldBeGreaterThan, 0)
					assertHighestProbability(artifact, testCase.wantCategory)
				})
			})
		}
	})
}

func BenchmarkLogitScoresRead(testingTB *testing.B) {
	config := logitScoresConfig()

	stage := NewLogitScores(config)
	artifact := datura.Acquire("logit-scores-bench-test", datura.APPJSON)
	artifact.Poke("output", "root")
	artifact.Poke([]string{"rvol", "precursor", "compression", "ignition", "value", "rvolDecline"}, "inputs")
	artifact.WithPayload(datura.Map[any]{
		"output": datura.Map[any]{
			"rvol":        1.2,
			"precursor":   0.05,
			"value":       0.5,
			"ignition":    0.1,
			"rvolDecline": 0.5,
		},
	}.Marshal())

	for testingTB.Loop() {
		_ = nomagique.RoundTripArtifact(artifact, stage)
	}
}
