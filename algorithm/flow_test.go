package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestFlowMeasure(testingTB *testing.T) {
	Convey("Given aggressive buy flow with rising price", testingTB, func() {
		flow := equation.NewFlow()
		output, err := flow.Measure(equation.FlowInput{
			BuyNotional:    500,
			TradeCount:     5,
			MedianNotional: 100,
			Prices:         []float64{100, 100.01, 100.02, 100.03, 100.04},
		})

		So(err, ShouldBeNil)

		Convey("It should favor aggressive drive", func() {
			So(output.Drive, ShouldBeGreaterThan, 0)
			So(output.Drive, ShouldBeGreaterThan, output.Absorption)
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

		Convey("It should favor hidden absorption", func() {
			So(output.Absorption, ShouldBeGreaterThan, 0)
		})
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
