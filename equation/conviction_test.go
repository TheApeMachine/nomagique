package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func TestConviction_Read(testingTB *testing.T) {
	Convey("Given broad positive breadth with leadership", testingTB, func() {
		stage := equation.NewConviction(nil)
		writeErr := writeFeatureStage(stage, equation.ConvictionInputKeys, 1.0, 2.0, 0.5, 1, 2.0)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify risk-on surge", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 1)
		})
	})

	Convey("Given broad positive breadth without leadership", testingTB, func() {
		stage := equation.NewConviction(nil)
		writeErr := writeFeatureStage(stage, equation.ConvictionInputKeys, 1.0, 0.1, 0.5, 0, 0.1)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should not classify risk-on surge without a leader", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldNotEqual, 1)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
		})
	})

	Convey("Given a local leader in a weak market", testingTB, func() {
		stage := equation.NewConviction(nil)
		writeErr := writeFeatureStage(stage, equation.ConvictionInputKeys, 0.33, 4.0, 0.5, 1, 4.0)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify divergent move", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
		})
	})

	Convey("Given weak breadth without leadership", testingTB, func() {
		stage := equation.NewConviction(nil)
		writeErr := writeFeatureStage(stage, equation.ConvictionInputKeys, 0.2, -1.0, 0.5, 0, -1.0)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify systemic slump", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
		})
	})
}

func BenchmarkConvictionRead(b *testing.B) {
	stage := equation.NewConviction(nil)

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.ConvictionInputKeys, 1.0, 2.0, 0.5, 1, 2.0)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
