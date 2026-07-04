package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestBookflow_Measure(testingTB *testing.T) {
	Convey("Given a bid-heavy book snapshot", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(bookflowInput(
			0.85, 0.80, 0.86, true, 0.8,
		))

		So(err, ShouldBeNil)

		Convey("It should classify loaded imbalance", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.Value, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given ask-heavy loaded depth confirmed by sell pressure", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(bookflowInput(
			-0.85, -0.80, -0.86, true, -5.0,
		))

		So(err, ShouldBeNil)

		Convey("It should boost loaded score in the same direction", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.LoadedScore, ShouldBeGreaterThan, 0.85)
		})
	})

	Convey("Given loaded depth opposed by trade pressure", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(bookflowInput(
			0.85, 0.80, 0.86, true, -5.0,
		))

		So(err, ShouldBeNil)

		Convey("It should damp loaded score without erasing the category evidence", func() {
			So(int(output.Category), ShouldEqual, 1)
			So(output.LoadedScore, ShouldBeGreaterThan, 0)
			So(output.LoadedScore, ShouldBeLessThan, 0.85)
		})
	})

	Convey("Given deep bid wall with bearish touch", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(equation.BookflowInput{
			Weighted:        0.6,
			Level1:          -0.4,
			Flat:            0.5,
			FlatOK:          true,
			Mid:             50,
			Spread:          2,
			TouchDepth:      3,
			TradePressure:   -0.5,
			WeightedHistory: []float64{0.6, 0.55, 0.58, 0.62},
			Level1History:   []float64{0.2, 0.18, 0.22, 0.19},
			FlatHistory:     []float64{0.25, 0.24, 0.26, 0.23},
		})

		So(err, ShouldBeNil)

		Convey("It should classify spoof trap", func() {
			So(int(output.Category), ShouldEqual, 2)
		})
	})

	Convey("Given weighted depth that collapses away from flat depth", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(equation.BookflowInput{
			Weighted:        0.8,
			Level1:          0.7,
			Flat:            0.1,
			FlatOK:          true,
			Mid:             100,
			Spread:          2,
			TouchDepth:      12,
			TradePressure:   0.2,
			WeightedHistory: []float64{0.60, 0.62, 0.61, 0.63},
			Level1History:   []float64{0.60, 0.62, 0.61, 0.63},
			FlatHistory:     []float64{0.50, 0.48, 0.52, 0.50},
		})

		So(err, ShouldBeNil)

		Convey("It should classify book thinning", func() {
			So(int(output.Category), ShouldEqual, 3)
			So(output.ThinScore, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given balanced depth below the loaded threshold", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(equation.BookflowInput{
			Weighted:        0.1,
			Level1:          0.05,
			Flat:            0.12,
			FlatOK:          true,
			Mid:             100,
			Spread:          2,
			TouchDepth:      12,
			WeightedHistory: []float64{0.50, 0.52, 0.51, 0.53},
			Level1History:   []float64{0.45, 0.46, 0.44, 0.47},
			FlatHistory:     []float64{0.50, 0.51, 0.49, 0.52},
		})

		So(err, ShouldBeNil)

		Convey("It should classify dense neutrality", func() {
			So(int(output.Category), ShouldEqual, 4)
			So(output.NeutralScore, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given incomplete features without bookflow evidence", testingTB, func() {
		bookflow := equation.NewBookflow()
		output, err := bookflow.Measure(equation.BookflowInput{})

		So(err, ShouldBeNil)

		Convey("It should emit zero category evidence", func() {
			So(output.Category, ShouldEqual, 0)
			So(output.Value, ShouldEqual, 0)
			So(output.LoadedScore, ShouldEqual, 0)
			So(output.SpoofScore, ShouldEqual, 0)
			So(output.ThinScore, ShouldEqual, 0)
			So(output.NeutralScore, ShouldEqual, 0)
		})
	})
}

func BenchmarkBookflowMeasure(benchmark *testing.B) {
	bookflow := equation.NewBookflow()
	input := bookflowInput(0.85, 0.80, 0.86, true, 0.8)

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = bookflow.Measure(input)
	}
}

func bookflowInput(
	weighted float64,
	level1 float64,
	flat float64,
	flatOK bool,
	tradePressure float64,
) equation.BookflowInput {
	return equation.BookflowInput{
		Weighted:        weighted,
		Level1:          level1,
		Flat:            flat,
		FlatOK:          flatOK,
		Mid:             100,
		Spread:          2,
		TouchDepth:      12,
		TradePressure:   tradePressure,
		WeightedHistory: []float64{0.80, 0.82, 0.84, 0.86},
		Level1History:   []float64{0.78, 0.79, 0.80, 0.81},
		FlatHistory:     []float64{0.80, 0.82, 0.83, 0.84},
	}
}
