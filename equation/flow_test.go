package equation_test

import (
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestFlow_Read(testingTB *testing.T) {
	Convey("Given aggressive buy flow with rising price", testingTB, func() {
		stage := equation.NewFlow(equation.FlowConfig())
		err := writeFeatureStage(stage, equation.FlowInputKeys,
			500, 0, 5, 0, 100,
			100, 100.01, 100.02, 100.03, 100.04,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify aggressive drive", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
			So(datura.Peek[float64](outbound, "output", "drive"), ShouldAlmostEqual, 1, 0.001)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldAlmostEqual, 1, 0.001)
		})
	})

	Convey("Given aggressive buy flow with flat price", testingTB, func() {
		stage := equation.NewFlow(equation.FlowConfig())
		err := writeFeatureStage(stage, equation.FlowInputKeys,
			200, 0, 4, 0, 50,
			50, 50.001, 50, 50.001,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify hidden absorption", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "absorption"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestFlow_ReadClassifiedCategories(testingTB *testing.T) {
	Convey("Given controlled trade-flow feature batches", testingTB, func() {
		type flowCase struct {
			name         string
			values       []float64
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

		cases := []flowCase{
			{
				name: "hidden absorption",
				values: []float64{
					200, 0, 4, 0, 50,
					50, 50.001, 50, 50.001,
				},
				wantCategory: 1,
				wantScore:    "absorption",
			},
			{
				name: "aggressive drive",
				values: []float64{
					500, 0, 5, 0, 100,
					100, 100.01, 100.02, 100.03, 100.04,
				},
				wantCategory: 2,
				wantScore:    "drive",
			},
			{
				name: "balanced flow",
				values: []float64{
					250, 250, 5, 0, 100,
					100, 100.01, 100.02, 100.03, 100.04,
				},
				wantCategory: 3,
				wantScore:    "balance",
			},
			{
				name: "flow starvation",
				values: []float64{
					100, 100, 2, 0, 100,
					100, 100.01,
				},
				wantCategory: 4,
				wantScore:    "starvation",
			},
		}

		for _, testCase := range cases {
			testCase := testCase

			Convey("When classifying "+testCase.name, func() {
				stage := nomagique.Number(
					equation.NewFlow(equation.FlowConfig()),
					probability.NewClassifier(
						datura.Acquire("flow-classifier", datura.APPJSON).Poke(
							[]string{"absorption", "drive", "balance", "starvation"},
							"inputs",
						),
					),
				)
				err := writeFeatureStage(stage, equation.FlowInputKeys, testCase.values...)

				So(err, ShouldBeNil)

				outbound, err := readStageOutput(stage)

				So(err, ShouldBeNil)

				Convey("It should put the intended category on top", func() {
					So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, testCase.wantCategory)
					So(datura.Peek[float64](outbound, "output", testCase.wantScore), ShouldBeGreaterThan, 0)
					assertHighestProbability(outbound, testCase.wantCategory)
				})
			})
		}
	})
}

func BenchmarkFlowRead(b *testing.B) {
	stage := equation.NewFlow(equation.FlowConfig())
	values := []float64{
		500, 0, 5, 0, 100,
		100, 100.01, 100.02, 100.03, 100.04,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.FlowInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
