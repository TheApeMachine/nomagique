package equation_test

import (
	"testing"

	"github.com/theapemachine/nomagique/equation"
)

func TestCausalStoryMeasure(testingTB *testing.T) {
	cases := []struct {
		name     string
		input    equation.CausalStoryInput
		category int
		score    func(equation.CausalStoryOutput) float64
	}{
		{
			name: "endogenous uplift",
			input: equation.CausalStoryInput{
				Association:  0.2,
				Intervention: 0.6,
				Uplift:       0.8,
				Contagion:    0.1,
				Condition:    0.2,
			},
			category: 1,
			score: func(output equation.CausalStoryOutput) float64 {
				return output.AlphaScore
			},
		},
		{
			name: "systemic beta",
			input: equation.CausalStoryInput{
				Association:  0.9,
				Intervention: 0.1,
				Uplift:       0.0,
				Contagion:    0.1,
				Condition:    0.2,
			},
			category: 2,
			score: func(output equation.CausalStoryOutput) float64 {
				return output.BetaScore
			},
		},
		{
			name: "liquidity shock",
			input: equation.CausalStoryInput{
				Association:  0.3,
				Intervention: 0.2,
				Uplift:       0.1,
				Contagion:    3.0,
				Condition:    0.2,
				Inverted:     true,
			},
			category: 3,
			score: func(output equation.CausalStoryOutput) float64 {
				return output.ShockScore
			},
		},
		{
			name: "causal noise",
			input: equation.CausalStoryInput{
				Association:  1.0,
				Intervention: 1.0,
				Uplift:       1.0,
				Contagion:    0.1,
				Condition:    0.2,
			},
			category: 4,
			score: func(output equation.CausalStoryOutput) float64 {
				return output.NoiseScore
			},
		},
	}

	for _, testCase := range cases {
		testingTB.Run(testCase.name, func(t *testing.T) {
			stage := equation.NewCausalStory()
			output, err := stage.Measure(testCase.input)
			if err != nil {
				t.Fatal(err)
			}

			if !output.Ready {
				t.Fatal("expected ready category")
			}

			if testCase.score(output) <= 0 {
				t.Fatalf("%s score is not positive", testCase.name)
			}

			if output.Category != testCase.category {
				t.Fatalf("category = %d, want %d", output.Category, testCase.category)
			}
		})
	}
}

func TestCausalStoryNoCategoryEvidence(testingTB *testing.T) {
	stage := equation.NewCausalStory()
	output, err := stage.Measure(equation.CausalStoryInput{
		Association:  0.0,
		Intervention: 1.0,
		Uplift:       0.0,
		Contagion:    0.0,
		Condition:    0.0,
	})
	if err != nil {
		testingTB.Fatal(err)
	}

	if output.Ready {
		testingTB.Fatal("expected not ready output")
	}
}

func TestCausalStoryTiedCategoryEvidence(testingTB *testing.T) {
	stage := equation.NewCausalStory()
	output, err := stage.Measure(equation.CausalStoryInput{
		Association:  1.0,
		Intervention: 1.0,
		Uplift:       1.0,
		Contagion:    0.4,
		Condition:    0.1,
		Inverted:     true,
	})
	if err != nil {
		testingTB.Fatal(err)
	}

	if output.Ready {
		testingTB.Fatal("expected tied evidence to be not ready")
	}
}

func BenchmarkCausalStoryMeasure(testingTB *testing.B) {
	stage := equation.NewCausalStory()
	input := equation.CausalStoryInput{
		Association:  0.2,
		Intervention: 0.6,
		Uplift:       0.8,
		Contagion:    0.1,
		Condition:    0.2,
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := stage.Measure(input); err != nil {
			testingTB.Fatal(err)
		}
	}
}
