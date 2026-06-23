package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestBookQuality_Read(testingTB *testing.T) {
	Convey("Given near-touch toxic churn above gate", testingTB, func() {
		stage := equation.NewBookQuality(nil)
		writeErr := writeFeatureStage(stage, equation.BookQualityInputKeys,
			0, 0.1, 0, 0.1,
			80, 80,
			1, 4.5,
			0.15, 0.8, 0, 2,
			100,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify toxic bluff", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 4.5)
		})
	})

	Convey("Given balanced depth with fills and no cancels", testingTB, func() {
		stage := equation.NewBookQuality(nil)
		writeErr := writeFeatureStage(stage, equation.BookQualityInputKeys,
			0, 0.1, 0, 0.1,
			80, 80,
			0, 0,
			0.15, 0, 0, 1,
			100,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify hard support", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 1)
		})
	})

	Convey("Given balanced depth without category evidence", testingTB, func() {
		stage := transport.NewPipeline(
			equation.NewBookQuality(nil),
			probability.NewClassifier(
				datura.Acquire("toxicity-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
					"inputs": []string{"bluffScore", "vacuumScore", "supportScore"},
				}),
			),
		)
		writeErr := writeFeatureStage(stage, equation.BookQualityInputKeys,
			0, 0, 0, 0,
			80, 80,
			0, 0,
			0.15, 0, 0, 1,
			100,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should emit uniform neutral confidence instead of rejecting", func() {
			So(datura.Peek[float64](outbound, "output", "confidence"), ShouldAlmostEqual, 1.0/3.0)
			So(datura.Peek[float64](outbound, "output", "strength"), ShouldEqual, 0)
		})
	})

	Convey("Given cancel/fill evidence before the adaptive threshold is ready", testingTB, func() {
		stage := transport.NewPipeline(
			equation.NewBookQuality(nil),
			probability.NewClassifier(
				datura.Acquire("toxicity-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
					"inputs": []string{"bluffScore", "vacuumScore", "supportScore"},
				}),
			),
		)
		writeErr := writeFeatureStage(stage, equation.BookQualityInputKeys,
			0.3, 0.1, 0, 0,
			80, 80,
			0, 0,
			0, 0, 0, 1,
			100,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should remain neutral instead of rejecting threshold warmup", func() {
			So(datura.Peek[float64](outbound, "output", "confidence"), ShouldAlmostEqual, 1.0/3.0)
			So(datura.Peek[float64](outbound, "output", "strength"), ShouldEqual, 0)
		})
	})
}

func BenchmarkBookQualityRead(b *testing.B) {
	stage := equation.NewBookQuality(nil)
	values := []float64{
		0.3, 0.1, 0, 0,
		10, 10,
		0, 0,
		0.15, 0, 0, 2,
		50000,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.BookQualityInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}

func BenchmarkBookQualityReadNeutral(b *testing.B) {
	stage := equation.NewBookQuality(nil)
	values := []float64{
		0, 0, 0, 0,
		10, 10,
		0, 0,
		0.15, 0, 0, 2,
		50000,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.BookQualityInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
