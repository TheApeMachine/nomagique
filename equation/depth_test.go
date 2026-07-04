package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestDepth_Measure(testingTB *testing.T) {
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
			So(int(output.Category), ShouldEqual, 3)
			So(output.Value, ShouldAlmostEqual, 0.3333333333333333, 0.001)
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
			So(output.ScarcityScore, ShouldBeGreaterThan, 0)
			So(output.ScarcityScore, ShouldBeLessThan, 1)
		})
	})

	Convey("Given an incomplete depth feature batch", testingTB, func() {
		depth := equation.NewDepth()
		_, err := depth.Measure(equation.DepthInput{
			QuoteVolume: 1000,
			Peers:       []float64{1000},
		})

		Convey("It should reject incomplete payload", func() {
			So(err, ShouldNotBeNil)
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
