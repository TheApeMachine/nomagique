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

func TestBookflow_Read(testingTB *testing.T) {
	Convey("Given a bid-heavy book snapshot", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.85, 0.80, 0.86, 1,
			100, 2, 12,
			0.8,
			4, 4, 4,
			0.80, 0.82, 0.84, 0.86,
			0.78, 0.79, 0.80, 0.81,
			0.80, 0.82, 0.83, 0.84,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify loaded imbalance", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given ask-heavy loaded depth confirmed by sell pressure", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			-0.85, -0.80, -0.86, 1,
			100, 2, 12,
			-5.0,
			4, 4, 4,
			-0.80, -0.82, -0.84, -0.86,
			-0.78, -0.79, -0.80, -0.81,
			-0.80, -0.82, -0.83, -0.84,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should boost loaded score in the same direction", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "loadedScore"), ShouldBeGreaterThan, 0.85)
		})
	})

	Convey("Given loaded depth opposed by trade pressure", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.85, 0.80, 0.86, 1,
			100, 2, 12,
			-5.0,
			4, 4, 4,
			0.80, 0.82, 0.84, 0.86,
			0.78, 0.79, 0.80, 0.81,
			0.80, 0.82, 0.83, 0.84,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should damp loaded score without erasing the category evidence", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "loadedScore"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](outbound, "output", "loadedScore"), ShouldBeLessThan, 0.85)
		})
	})

	Convey("Given deep bid wall with bearish touch", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.6, -0.4, 0.5, 1,
			50, 2, 3,
			-0.5,
			4, 4, 4,
			0.6, 0.55, 0.58, 0.62,
			0.2, 0.18, 0.22, 0.19,
			0.25, 0.24, 0.26, 0.23,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify spoof trap", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
		})
	})

	Convey("Given weighted depth that collapses away from flat depth", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.8, 0.7, 0.1, 1,
			100, 2, 12,
			0.2,
			4, 4, 4,
			0.60, 0.62, 0.61, 0.63,
			0.60, 0.62, 0.61, 0.63,
			0.50, 0.48, 0.52, 0.50,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify book thinning", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
			So(datura.Peek[float64](outbound, "output", "thinScore"), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given balanced depth below the loaded threshold", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.1, 0.05, 0.12, 1,
			100, 2, 12,
			0.0,
			4, 4, 4,
			0.50, 0.52, 0.51, 0.53,
			0.45, 0.46, 0.44, 0.47,
			0.50, 0.51, 0.49, 0.52,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify dense neutrality", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 4)
			So(datura.Peek[float64](outbound, "output", "neutralScore"), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given valid features without bookflow evidence", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0, 0, 0, 0,
			0, 0, 0,
			0,
			0, 0, 0,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should emit zero category evidence", func() {
			So(datura.Peek[float64](outbound, "output", "category"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "loadedScore"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "spoofScore"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "thinScore"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "neutralScore"), ShouldEqual, 0)
		})
	})
}

func TestBookflow_ReadClassifiedCategories(testingTB *testing.T) {
	Convey("Given controlled bookflow feature batches", testingTB, func() {
		type bookflowCase struct {
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

		cases := []bookflowCase{
			{
				name: "loaded imbalance",
				values: []float64{
					0.85, 0.80, 0.86, 1,
					100, 2, 12,
					0.8,
					4, 4, 4,
					0.80, 0.82, 0.84, 0.86,
					0.78, 0.79, 0.80, 0.81,
					0.80, 0.82, 0.83, 0.84,
				},
				wantCategory: 1,
				wantScore:    "loadedScore",
			},
			{
				name: "spoof trap",
				values: []float64{
					0.6, -0.4, 0.5, 1,
					50, 2, 3,
					-0.5,
					4, 4, 4,
					0.6, 0.55, 0.58, 0.62,
					0.2, 0.18, 0.22, 0.19,
					0.25, 0.24, 0.26, 0.23,
				},
				wantCategory: 2,
				wantScore:    "spoofScore",
			},
			{
				name: "book thinning",
				values: []float64{
					0.8, 0.7, 0.1, 1,
					100, 2, 12,
					0.2,
					4, 4, 4,
					0.60, 0.62, 0.61, 0.63,
					0.60, 0.62, 0.61, 0.63,
					0.50, 0.48, 0.52, 0.50,
				},
				wantCategory: 3,
				wantScore:    "thinScore",
			},
			{
				name: "dense neutrality",
				values: []float64{
					0.1, 0.05, 0.12, 1,
					100, 2, 12,
					0.0,
					4, 4, 4,
					0.50, 0.52, 0.51, 0.53,
					0.45, 0.46, 0.44, 0.47,
					0.50, 0.51, 0.49, 0.52,
				},
				wantCategory: 4,
				wantScore:    "neutralScore",
			},
		}

		for _, testCase := range cases {
			testCase := testCase

			Convey("When classifying "+testCase.name, func() {
				stage := nomagique.Number(
					equation.NewBookflow(bookflowConfig()),
					probability.NewClassifier(
						datura.Acquire("bookflow-classifier", datura.APPJSON).Poke(
							[]string{"loadedScore", "spoofScore", "thinScore", "neutralScore"},
							"inputs",
						),
					),
				)
				err := writeFeatureStage(stage, equation.BookflowInputKeys, testCase.values...)

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

func BenchmarkBookflowRead(b *testing.B) {
	stage := equation.NewBookflow(bookflowConfig())
	values := []float64{
		0.85, 0.80, 0.86, 1,
		100, 2, 12,
		0.8,
		4, 4, 4,
		0.80, 0.82, 0.84, 0.86,
		0.78, 0.79, 0.80, 0.81,
		0.80, 0.82, 0.83, 0.84,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.BookflowInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
