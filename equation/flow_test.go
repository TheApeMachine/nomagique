package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func TestFlow_Read(testingTB *testing.T) {
	Convey("Given aggressive buy flow with rising price", testingTB, func() {
		stage := equation.NewFlow(nil)
		writeErr := writeFeatureStage(stage, equation.FlowInputKeys,
			500, 0, 5, 0, 100,
			100, 100.01, 100.02, 100.03, 100.04,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify aggressive drive", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
			So(datura.Peek[float64](outbound, "output", "drive"), ShouldAlmostEqual, 1, 0.001)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldAlmostEqual, 1, 0.001)
		})
	})

	Convey("Given aggressive buy flow with flat price", testingTB, func() {
		stage := equation.NewFlow(nil)
		writeErr := writeFeatureStage(stage, equation.FlowInputKeys,
			200, 0, 4, 0, 50,
			50, 50.001, 50, 50.001,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify hidden absorption", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "absorption"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkFlowRead(b *testing.B) {
	stage := equation.NewFlow(nil)
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
