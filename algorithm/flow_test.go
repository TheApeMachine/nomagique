package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/tests"
)

func TestFlowEvaluate(testingTB *testing.T) {
	Convey("Given aggressive buy flow with rising price", testingTB, func() {
		flow := NewFlow()
		writeErr := tests.WriteSamples(flow,
			500, 0, 5, 0, 100,
			100, 100.01, 100.02, 100.03, 100.04,
		)
		So(writeErr, ShouldBeNil)
		_, _ = flow.Read(make([]byte, 4096))

		Convey("It should favor aggressive drive", func() {
			So(flow.outcome.Drive, ShouldBeGreaterThan, 0)
			So(flow.outcome.Drive, ShouldBeGreaterThan, flow.outcome.Absorption)
		})
	})

	Convey("Given aggressive buy flow with flat price", testingTB, func() {
		flow := NewFlow()
		writeErr := tests.WriteSamples(flow,
			200, 0, 4, 0, 50,
			50, 50.001, 50, 50.001,
		)
		So(writeErr, ShouldBeNil)
		_, _ = flow.Read(make([]byte, 4096))

		Convey("It should favor hidden absorption", func() {
			So(flow.outcome.Absorption, ShouldBeGreaterThan, 0)
		})
	})
}

func TestFlowClassifier(testingTB *testing.T) {
	Convey("Given a flow stage wired into a classifier", testingTB, func() {
		flow := NewFlow()
		classifier := probability.NewClassifier(
			flow.AbsorptionReading(),
			flow.DriveReading(),
			flow.BalanceReading(),
			flow.StarvationReading(),
		)
		pipeline := nomagique.Number(flow, classifier)
		writeErr := tests.WriteSamples(pipeline,
			500, 0, 5, 0, 100,
			100, 100.01, 100.02, 100.03, 100.04,
		)
		So(writeErr, ShouldBeNil)
		_, _ = pipeline.Read(make([]byte, 4096))

		Convey("It should select a category", func() {
			So(classifier.CategoryIndex(), ShouldBeGreaterThan, 0)

			confidence, confidenceErr := classifier.Confidence(classifier.CategoryIndex())

			So(confidenceErr, ShouldBeNil)
			So(confidence, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkFlowRead(b *testing.B) {
	flow := NewFlow()
	samples := []float64{
		500, 0, 5, 0, 100,
		100, 100.01, 100.02, 100.03, 100.04,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(flow, samples...)
		_, _ = flow.Read(make([]byte, 4096))
	}
}

func BenchmarkFlowPipeline(b *testing.B) {
	flow := NewFlow()
	classifier := probability.NewClassifier(
		flow.AbsorptionReading(),
		flow.DriveReading(),
		flow.BalanceReading(),
		flow.StarvationReading(),
	)
	pipeline := nomagique.Number(flow, classifier)
	samples := []float64{
		500, 0, 5, 0, 100,
		100, 100.01, 100.02, 100.03, 100.04,
	}
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(pipeline, samples...)
		_, _ = pipeline.Read(frame)
	}
}
