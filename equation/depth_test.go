package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func TestDepth_Read(testingTB *testing.T) {
	Convey("Given deep quote volume versus peers", testingTB, func() {
		stage := equation.NewDepth(nil)
		writeErr := writeFeatureStage(stage, equation.DepthInputKeys,
			1200, 4,
			800, 900, 1000, 1100,
			1, 0,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify robust liquidity", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldAlmostEqual, 0.3333333333333333, 0.001)
		})
	})

	Convey("Given peak scarcity volume", testingTB, func() {
		stage := equation.NewDepth(nil)
		writeErr := writeFeatureStage(stage, equation.DepthInputKeys,
			50, 3,
			1100, 950, 50,
			1, 0,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify extreme scarcity", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "scarcityScore"), ShouldEqual, 1)
		})
	})
}

func BenchmarkDepthRead(b *testing.B) {
	stage := equation.NewDepth(nil)
	values := []float64{
		1200, 4,
		800, 900, 1000, 1100,
		1, 0,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.DepthInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
