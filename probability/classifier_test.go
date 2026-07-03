package probability_test

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
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
	Convey("Given no inbound frame", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1"))

		Convey("It should report EOF without manufacturing state", func() {
			buffer := make([]byte, 1024)
			n, err := classifier.Read(buffer)

			So(n, ShouldEqual, 0)
			So(err, ShouldEqual, io.EOF)
		})
	})

	Convey("Given an empty inbound write", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1"))
		written, err := classifier.Write(nil)

		So(written, ShouldEqual, 0)
		So(err, ShouldBeNil)

		Convey("It should still report EOF on read", func() {
			buffer := make([]byte, 1024)
			n, err := classifier.Read(buffer)

			So(n, ShouldEqual, 0)
			So(err, ShouldEqual, io.EOF)
		})
	})

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
		err := nomagique.RoundTripArtifact(artifact, classifier)

		So(err, ShouldBeNil)

		Convey("It should return a 1-based winning category index", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 3)
			So(int(datura.Peek[float64](artifact, "output", "category")), ShouldEqual, 3)
		})

		Convey("It should expose normalized probabilities on the artifact", func() {
			confidence := datura.Peek[float64](artifact, "output", "confidence")
			distribution := datura.Peek[map[string]any](artifact, "output", "distribution")

			So(confidence, ShouldBeGreaterThan, 0)
			So(confidence, ShouldBeLessThan, 1)
			So(distribution["1"], ShouldAlmostEqual, datura.Peek[[]float64](artifact, "output", "probabilities")[0], 1e-12)
			So(distribution["2"], ShouldAlmostEqual, datura.Peek[[]float64](artifact, "output", "probabilities")[1], 1e-12)
			So(distribution["3"], ShouldAlmostEqual, datura.Peek[[]float64](artifact, "output", "probabilities")[2], 1e-12)
			So(distribution["4"], ShouldAlmostEqual, datura.Peek[[]float64](artifact, "output", "probabilities")[3], 1e-12)
		})

		Convey("It should expose the no-edge confidence baseline from category count", func() {
			So(datura.Peek[float64](artifact, "output", "confidence_baseline"), ShouldAlmostEqual, 0.25, 1e-12)
			So(datura.Peek[float64](artifact, "output", "entry_baseline"), ShouldAlmostEqual, 0.25, 1e-12)
			So(datura.Peek[float64](artifact, "output", "exit_baseline"), ShouldAlmostEqual, 0.25, 1e-12)
		})

		Convey("It should propagate score keys on output wire", func() {
			inputs := datura.Peek[[]string](artifact, "inputs")

			So(inputs, ShouldContain, "s0")
			So(inputs, ShouldContain, "s1")
			So(inputs, ShouldContain, "s2")
			So(inputs, ShouldContain, "s3")
			So(inputs, ShouldContain, "category")
			So(inputs, ShouldContain, "confidence_baseline")
			So(inputs, ShouldContain, "distribution")
			So(inputs, ShouldContain, "entry_baseline")
			So(inputs, ShouldContain, "exit_baseline")
			So(inputs, ShouldContain, "value")
		})
	})

	Convey("Given schema attributes are mutated after construction", testingTB, func() {
		schema := classifierSchema("s0", "s1", "s2")
		classifier := probability.NewClassifier(schema)
		schema.Poke([]string{}, "inputs")

		artifact := artifactWithScores(map[string]float64{
			"s0":       0.1,
			"s1":       0.8,
			"s2":       0.2,
			"strength": 0.8,
		})

		err := nomagique.RoundTripArtifact(artifact, classifier)

		Convey("It should classify using the construction-time category schema", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 2)
		})
	})

	Convey("Given an empty score key in schema inputs", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", ""))

		So(classifier, ShouldNotBeNil)

		artifact := artifactWithScores(map[string]float64{
			"s0": 1,
		})
		err := nomagique.RoundTripArtifact(artifact, classifier)

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
		err := nomagique.RoundTripArtifact(artifact, classifier)

		So(err, ShouldBeNil)

		Convey("It should emit uniform confidence without error", func() {
			So(datura.Peek[float64](artifact, "output", "confidence"), ShouldAlmostEqual, 0.25, 1e-9)
			So(datura.Peek[float64](artifact, "output", "confidence_baseline"), ShouldAlmostEqual, 0.25, 1e-12)
			So(datura.Peek[float64](artifact, "output", "entry_baseline"), ShouldAlmostEqual, 0.25, 1e-12)
			So(datura.Peek[float64](artifact, "output", "exit_baseline"), ShouldAlmostEqual, 0.25, 1e-12)
			So(datura.Peek[float64](artifact, "output", "strength"), ShouldEqual, 0)
		})
	})

	Convey("Given a short output buffer", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1", "s2"))
		artifact := artifactWithScores(map[string]float64{
			"s0":       0.1,
			"s1":       0.8,
			"s2":       0.2,
			"strength": 0.8,
		})

		written, err := classifier.Write(artifact.Pack())

		So(written, ShouldBeGreaterThan, 0)
		So(err, ShouldBeNil)

		small := make([]byte, 1)
		n, err := classifier.Read(small)

		So(n, ShouldEqual, 1)
		So(err, ShouldEqual, io.ErrShortBuffer)

		large := make([]byte, 64*1024)
		n, err = classifier.Read(large)

		So(n, ShouldBeGreaterThan, 1)
		So(err, ShouldEqual, io.EOF)

		output := datura.Acquire("classifier-short-buffer-output", datura.APPJSON)
		_, err = output.Unpack(large[:n])

		So(err, ShouldBeNil)
		So(datura.Peek[float64](output, "output", "value"), ShouldEqual, 2)
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
		err := nomagique.RoundTripArtifact(artifact, pipeline)

		So(err, ShouldBeNil)

		Convey("It should return the winning category as float64", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 2)
			So(datura.Peek[float64](artifact, "output", "confidence_baseline"), ShouldAlmostEqual, 1.0/3.0, 1e-12)
			So(datura.Peek[float64](artifact, "output", "entry_baseline"), ShouldAlmostEqual, 1.0/3.0, 1e-12)
			So(datura.Peek[float64](artifact, "output", "exit_baseline"), ShouldAlmostEqual, 1.0/3.0, 1e-12)
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
		_ = nomagique.RoundTripArtifact(artifact, classifier)
	}
}
