package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/tests"
)

func TestConvictionEvaluate(testingTB *testing.T) {
	Convey("Given broad positive breadth", testingTB, func() {
		conviction := NewConviction()
		writeErr := tests.WriteSamples(conviction, 1.0, 2.0, 0.5, 1, 2.0)
		So(writeErr, ShouldBeNil)
		_, _ = conviction.Read(make([]byte, 4096))

		Convey("It should classify risk-on surge", func() {
			So(conviction.outcome.Eligible, ShouldBeTrue)
			So(conviction.outcome.Category, ShouldEqual, 1)
		})
	})

	Convey("Given a local leader in a weak market", testingTB, func() {
		conviction := NewConviction()
		writeErr := tests.WriteSamples(conviction, 0.33, 4.0, 0.5, 1, 4.0)
		So(writeErr, ShouldBeNil)
		_, _ = conviction.Read(make([]byte, 4096))

		Convey("It should classify divergent move", func() {
			So(conviction.outcome.Category, ShouldEqual, 2)
		})
	})

	Convey("Given weak breadth without leadership", testingTB, func() {
		conviction := NewConviction()
		writeErr := tests.WriteSamples(conviction, 0.2, -1.0, 0.5, 0, -1.0)
		So(writeErr, ShouldBeNil)
		_, _ = conviction.Read(make([]byte, 4096))

		Convey("It should classify systemic slump", func() {
			So(conviction.outcome.Category, ShouldEqual, 3)
		})
	})
}

func TestConvictionClassifier(testingTB *testing.T) {
	Convey("Given a conviction stage wired into a classifier", testingTB, func() {
		conviction := NewConviction()
		classifier := probability.NewClassifier(
			conviction.SurgeReading(),
			conviction.DivergentReading(),
			conviction.SlumpReading(),
		)
		pipeline := nomagique.Number(conviction, classifier)
		writeErr := tests.WriteSamples(pipeline, 1.0, 2.0, 0.5, 1, 2.0)
		So(writeErr, ShouldBeNil)
		_, _ = pipeline.Read(make([]byte, 4096))

		Convey("It should select a category", func() {
			So(classifier.CategoryIndex(), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkConvictionRead(b *testing.B) {
	conviction := NewConviction()
	samples := []float64{1.0, 2.0, 0.5, 1, 2.0}

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(conviction, samples...)
		_, _ = conviction.Read(make([]byte, 4096))
	}
}
