package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestDepthMeasure(testingTB *testing.T) {
	Convey("Given deep quote volume versus peers", testingTB, func() {
		depth := equation.NewDepth()
		output, err := depth.Measure(equation.DepthInput{
			QuoteVolume:    1200,
			Peers:          []float64{800, 900, 1000, 1100},
			RelativeVolume: 1,
			BaselineReady:  false,
		})

		So(err, ShouldBeNil)

		Convey("It should classify robust liquidity", func() {
			So(output.Value, ShouldBeGreaterThan, 0)
			So(int(output.Category), ShouldEqual, 3)
		})
	})

	Convey("Given peak scarcity volume", testingTB, func() {
		depth := equation.NewDepth()
		output, err := depth.Measure(equation.DepthInput{
			QuoteVolume:    50,
			Peers:          []float64{1100, 950, 50},
			RelativeVolume: 1,
			BaselineReady:  false,
		})

		So(err, ShouldBeNil)

		Convey("It should classify extreme scarcity", func() {
			So(int(output.Category), ShouldEqual, 1)
		})
	})
}

func BenchmarkDepthMeasure(benchmark *testing.B) {
	depth := equation.NewDepth()
	input := equation.DepthInput{
		QuoteVolume:    1200,
		Peers:          []float64{800, 900, 1000, 1100},
		RelativeVolume: 1,
		BaselineReady:  false,
	}

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = depth.Measure(input)
	}
}
