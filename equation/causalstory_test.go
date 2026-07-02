package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/equation"
)

func TestCausalStory_Read(testingTB *testing.T) {
	cases := []struct {
		name         string
		association  float64
		intervention float64
		uplift       float64
		contagion    float64
		condition    float64
		inverted     float64
		category     float64
		score        string
	}{
		{
			name:         "endogenous uplift",
			association:  0.2,
			intervention: 0.6,
			uplift:       0.8,
			contagion:    0.1,
			condition:    0.2,
			category:     1,
			score:        "alphaScore",
		},
		{
			name:         "systemic beta",
			association:  0.9,
			intervention: 0.1,
			uplift:       0.0,
			contagion:    0.1,
			condition:    0.2,
			category:     2,
			score:        "betaScore",
		},
		{
			name:         "liquidity shock",
			association:  0.3,
			intervention: 0.2,
			uplift:       0.1,
			contagion:    3.0,
			condition:    0.2,
			inverted:     1.0,
			category:     3,
			score:        "shockScore",
		},
		{
			name:         "causal noise",
			association:  1.0,
			intervention: 1.0,
			uplift:       1.0,
			contagion:    0.1,
			condition:    0.2,
			category:     4,
			score:        "noiseScore",
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given Pearl ladder outputs with "+testCase.name, testingTB, func() {
			stage := equation.NewCausalStory(equation.CausalStoryConfig())
			inbound := datura.Acquire("causal-story-in", datura.APPJSON)
			inbound.MergeOutput("association", testCase.association)
			inbound.MergeOutput("intervention", testCase.intervention)
			inbound.MergeOutput("uplift", testCase.uplift)
			inbound.MergeOutput("contagion", testCase.contagion)
			inbound.MergeOutput("condition", testCase.condition)
			inbound.MergeOutput("inverted", testCase.inverted)

			err := nomagique.RoundTripArtifact(inbound, stage)

			So(err, ShouldBeNil)

			Convey("It should emit the expected causal category score", func() {
				So(datura.Peek[float64](inbound, "output", testCase.score), ShouldBeGreaterThan, 0)
				So(datura.Peek[float64](inbound, "output", "category"), ShouldEqual, testCase.category)
			})
		})
	}
}

func BenchmarkCausalStoryRead(b *testing.B) {
	stage := equation.NewCausalStory(equation.CausalStoryConfig())
	inbound := datura.Acquire("causal-story-bench", datura.APPJSON)
	inbound.MergeOutput("association", 0.2)
	inbound.MergeOutput("intervention", 0.6)
	inbound.MergeOutput("uplift", 0.8)
	inbound.MergeOutput("contagion", 0.1)
	inbound.MergeOutput("condition", 0.2)
	inbound.MergeOutput("inverted", 0.0)

	b.ReportAllocs()

	for b.Loop() {
		_ = nomagique.RoundTripArtifact(inbound, stage)
	}
}
