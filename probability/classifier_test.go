package probability_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
)

func classifierSchema(inputs ...string) *datura.Artifact {
	return datura.Acquire("schema", datura.APPJSON).
		Poke(inputs, "inputs")
}

func TestNewClassifier(testingTB *testing.T) {
	Convey("Given a schema artifact", testingTB, func() {
		schema := classifierSchema("s0", "s1")
		classifier := probability.NewClassifier(schema)

		Convey("It should store the schema artifact", func() {
			So(classifier, ShouldNotBeNil)
		})
	})
}

func TestClassifier_Read(testingTB *testing.T) {
	Convey("Given four output scores on the artifact", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1", "s2", "s3"))

		So(classifier, ShouldNotBeNil)

		artifact := artifactWithScores(map[string]float64{
			"s0":       0.2,
			"s1":       0.1,
			"s2":       0.9,
			"s3":       0.05,
			"strength": 0.9,
		})
		err := transport.NewFlipFlop(artifact, classifier)

		So(err, ShouldBeNil)

		Convey("It should return a 1-based winning category index", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 3)
			So(int(datura.Peek[float64](artifact, "output", "category")), ShouldEqual, 3)
		})

		Convey("It should expose normalized probabilities on the artifact", func() {
			confidence := datura.Peek[float64](artifact, "output", "confidence")

			So(confidence, ShouldBeGreaterThan, 0)
			So(confidence, ShouldBeLessThan, 1)
		})

		Convey("It should propagate score keys on output wire", func() {
			inputs := datura.Peek[[]string](artifact, "inputs")

			So(inputs, ShouldContain, "s0")
			So(inputs, ShouldContain, "s1")
			So(inputs, ShouldContain, "s2")
			So(inputs, ShouldContain, "s3")
			So(inputs, ShouldContain, "category")
			So(inputs, ShouldContain, "value")
		})
	})

	Convey("Given an empty score key in schema inputs", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", ""))

		So(classifier, ShouldNotBeNil)

		artifact := artifactWithScores(map[string]float64{
			"s0": 1,
		})
		err := transport.NewFlipFlop(artifact, classifier)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given zero category evidence and zero strength", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1", "s2", "s3"))

		So(classifier, ShouldNotBeNil)

		artifact := artifactWithScores(map[string]float64{
			"s0":       0,
			"s1":       0,
			"s2":       0,
			"s3":       0,
			"strength": 0,
		})
		err := transport.NewFlipFlop(artifact, classifier)

		So(err, ShouldBeNil)

		Convey("It should emit uniform confidence without error", func() {
			So(datura.Peek[float64](artifact, "output", "confidence"), ShouldAlmostEqual, 0.25, 1e-9)
			So(datura.Peek[float64](artifact, "output", "strength"), ShouldEqual, 0)
		})
	})
}

func TestClassifier_Number(testingTB *testing.T) {
	Convey("Given Number composed with Classifier", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1", "s2"))

		So(classifier, ShouldNotBeNil)

		artifact := artifactWithScores(map[string]float64{
			"s0":       0.1,
			"s1":       0.8,
			"s2":       0.2,
			"strength": 0.8,
		})
		pipeline := nomagique.Number(classifier)
		err := transport.NewFlipFlop(artifact, pipeline)

		So(err, ShouldBeNil)

		Convey("It should return the winning category as float64", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 2)
		})
	})
}

func BenchmarkClassifier_Read(b *testing.B) {
	classifier := probability.NewClassifier(classifierSchema("s0", "s1", "s2", "s3"))

	if classifier == nil {
		b.Fatal("classifier required")
	}

	artifact := artifactWithScores(map[string]float64{
		"s0":       0.2,
		"s1":       0.4,
		"s2":       0.7,
		"s3":       0.1,
		"strength": 0.7,
	})

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, classifier)
	}
}
