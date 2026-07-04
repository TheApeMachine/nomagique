package equation_test

import (
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestFlow_Measure(testingTB *testing.T) {
	Convey("Given aggressive buy flow with rising price", testingTB, func() {
		flow := equation.NewFlow()
		output, err := flow.Measure(equation.FlowInput{
			BuyNotional:    500,
			TradeCount:     5,
			MedianNotional: 100,
			Prices:         []float64{100, 100.01, 100.02, 100.03, 100.04},
		})

		So(err, ShouldBeNil)

		Convey("It should classify aggressive drive", func() {
			So(int(output.Category), ShouldEqual, 2)
			So(output.Drive, ShouldAlmostEqual, 1, 0.001)
			So(output.Value, ShouldAlmostEqual, 1, 0.001)
		})
	})

	Convey("Given aggressive buy flow with flat price", testingTB, func() {
		flow := equation.NewFlow()
		output, err := flow.Measure(equation.FlowInput{
			BuyNotional:    200,
			TradeCount:     4,
			MedianNotional: 50,
			Prices:         []float64{50, 50.001, 50, 50.001},
		})

		So(err, ShouldBeNil)

		Convey("It should classify hidden absorption", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.Absorption, ShouldBeGreaterThan, 0)
		})
	})
}

func TestFlow_MeasureClassifiedCategories(testingTB *testing.T) {
	Convey("Given controlled trade-flow inputs", testingTB, func() {
		type flowCase struct {
			name         string
			input        equation.FlowInput
			wantCategory int
			wantScore    func(equation.FlowOutput) float64
		}

		assertHighestProbability := func(output equation.FlowOutput, category int) {
			classifier := probability.NewScoreClassifier(
				[]string{"absorption", "drive", "balance", "starvation"},
				nil,
			)
			result, err := classifier.Classify(map[string]float64{
				"absorption": output.Absorption,
				"drive":      output.Drive,
				"balance":    output.Balance,
				"starvation": output.Starvation,
				"strength":   output.Value,
			})

			So(err, ShouldBeNil)
			So(len(result.Probabilities), ShouldEqual, 4)
			So(len(result.Distribution), ShouldEqual, 4)

			selected := result.Probabilities[category-1]
			total := 0.0

			for index, item := range result.Probabilities {
				total += item
				mass, ok := result.Distribution[strconv.Itoa(index+1)]
				So(ok, ShouldBeTrue)
				So(mass, ShouldAlmostEqual, item, 1e-12)
				if index != category-1 {
					So(selected, ShouldBeGreaterThan, item)
				}
			}

			So(total, ShouldAlmostEqual, 1.0, 1e-12)
		}

		cases := []flowCase{
			{
				name: "hidden absorption",
				input: equation.FlowInput{
					BuyNotional:    200,
					TradeCount:     4,
					MedianNotional: 50,
					Prices:         []float64{50, 50.001, 50, 50.001},
				},
				wantCategory: 1,
				wantScore:    func(output equation.FlowOutput) float64 { return output.Absorption },
			},
			{
				name: "aggressive drive",
				input: equation.FlowInput{
					BuyNotional:    500,
					TradeCount:     5,
					MedianNotional: 100,
					Prices:         []float64{100, 100.01, 100.02, 100.03, 100.04},
				},
				wantCategory: 2,
				wantScore:    func(output equation.FlowOutput) float64 { return output.Drive },
			},
			{
				name: "balanced flow",
				input: equation.FlowInput{
					BuyNotional:    250,
					SellNotional:   250,
					TradeCount:     5,
					MedianNotional: 100,
					Prices:         []float64{100, 100.01, 100.02, 100.03, 100.04},
				},
				wantCategory: 3,
				wantScore:    func(output equation.FlowOutput) float64 { return output.Balance },
			},
			{
				name: "flow starvation",
				input: equation.FlowInput{
					BuyNotional:    100,
					SellNotional:   100,
					TradeCount:     2,
					MedianNotional: 100,
					Prices:         []float64{100, 100.01},
				},
				wantCategory: 4,
				wantScore:    func(output equation.FlowOutput) float64 { return output.Starvation },
			},
		}

		for _, testCase := range cases {
			testCase := testCase

			Convey("When classifying "+testCase.name, func() {
				flow := equation.NewFlow()
				output, err := flow.Measure(testCase.input)

				So(err, ShouldBeNil)

				Convey("It should put the intended category on top", func() {
					So(int(output.Category), ShouldEqual, testCase.wantCategory)
					So(testCase.wantScore(output), ShouldBeGreaterThan, 0)
					assertHighestProbability(output, testCase.wantCategory)
				})
			})
		}
	})
}

func BenchmarkFlowMeasure(benchmark *testing.B) {
	flow := equation.NewFlow()
	input := equation.FlowInput{
		BuyNotional:    500,
		TradeCount:     5,
		MedianNotional: 100,
		Prices:         []float64{100, 100.01, 100.02, 100.03, 100.04},
	}

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = flow.Measure(input)
	}
}
