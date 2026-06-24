package probability_test

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
)

func softmaxSchema(inputs ...string) *datura.Artifact {
	return datura.Acquire("schema", datura.APPJSON).
		Poke("output", "scoreRoot").
		Poke(inputs, "inputs")
}

func TestNewSoftmax(testingTB *testing.T) {
	Convey("Given a schema artifact", testingTB, func() {
		softmax := probability.NewSoftmax(softmaxSchema("s0", "s1"))

		Convey("It should return a usable stage", func() {
			So(softmax, ShouldNotBeNil)
		})
	})
}

func TestSoftmax_Read(testingTB *testing.T) {
	Convey("Given four output scores on the artifact", testingTB, func() {
		softmax := probability.NewSoftmax(softmaxSchema("s0", "s1", "s2", "s3"))

		artifact := artifactWithScores(map[string]float64{
			"s0": 0.2,
			"s1": 0.1,
			"s2": 0.9,
			"s3": 0.05,
		})
		err := transport.NewFlipFlop(artifact, softmax)

		So(err, ShouldBeNil)

		Convey("It should expose every class probability", func() {
			probabilities := datura.Peek[[]float64](artifact, "output", "probabilities")

			So(len(probabilities), ShouldEqual, 4)

			sum := 0.0

			for _, share := range probabilities {
				sum += share
			}

			So(sum, ShouldAlmostEqual, 1.0, 1e-9)
			So(datura.Peek[float64](artifact, "output", "s2"), ShouldBeGreaterThan, 0.2)
			So(datura.Peek[float64](artifact, "output", "s0"), ShouldBeGreaterThan, 0)
		})

		Convey("It should propagate score keys on output wire", func() {
			inputs := datura.Peek[[]string](artifact, "inputs")

			So(inputs, ShouldContain, "s0")
			So(inputs, ShouldContain, "s1")
			So(inputs, ShouldContain, "s2")
			So(inputs, ShouldContain, "s3")
			So(inputs, ShouldContain, "probabilities")
		})
	})

	Convey("Given an empty score key in schema inputs", testingTB, func() {
		softmax := probability.NewSoftmax(softmaxSchema("s0", ""))
		artifact := artifactWithScores(map[string]float64{"s0": 1})
		err := transport.NewFlipFlop(artifact, softmax)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given uniform scores", testingTB, func() {
		softmax := probability.NewSoftmax(softmaxSchema("s0", "s1", "s2", "s3"))
		artifact := artifactWithScores(map[string]float64{
			"s0": 3,
			"s1": 3,
			"s2": 3,
			"s3": 3,
		})
		err := transport.NewFlipFlop(artifact, softmax)

		So(err, ShouldBeNil)

		Convey("It should emit the uniform distribution", func() {
			So(datura.Peek[float64](artifact, "output", "s0"), ShouldAlmostEqual, 0.25, 1e-9)
			So(datura.Peek[float64](artifact, "output", "s1"), ShouldAlmostEqual, 0.25, 1e-9)
		})
	})

	Convey("Given non-finite scores", testingTB, func() {
		softmax := probability.NewSoftmax(
			datura.Acquire("schema", datura.APPJSON).
				Poke("features", "scoreRoot").
				Poke([]string{"s0", "s1", "s2"}, "inputs"),
		)
		artifact := datura.Acquire("test", datura.APPJSON)
		artifact.Poke("features", "root")
		artifact.Poke([]string{"s0", "s1", "s2"}, "inputs")
		artifact.Merge("features", []float64{1, math.NaN(), 3})
		err := transport.NewFlipFlop(artifact, softmax)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given raw mode on the config artifact", testingTB, func() {
		schema := datura.Acquire("schema", datura.APPJSON).
			Poke("output", "scoreRoot").
			Poke([]string{"s0", "s1", "s2"}, "inputs").
			Poke(0.0, "normalize")
		softmax := probability.NewSoftmax(schema)
		artifact := artifactWithScores(map[string]float64{
			"s0": 1,
			"s1": 2,
			"s2": 3,
		})
		err := transport.NewFlipFlop(artifact, softmax)

		So(err, ShouldBeNil)

		Convey("It should rank the largest logit highest", func() {
			probabilities := datura.Peek[[]float64](artifact, "output", "probabilities")

			So(probability.ArgmaxIndex(probabilities), ShouldEqual, 2)
		})
	})
}

func TestSoftmax_Number(testingTB *testing.T) {
	Convey("Given Number composed with Softmax", testingTB, func() {
		softmax := probability.NewSoftmax(softmaxSchema("s0", "s1", "s2"))
		artifact := artifactWithScores(map[string]float64{
			"s0": 0.1,
			"s1": 0.8,
			"s2": 0.2,
		})
		pipeline := nomagique.Number(softmax)
		err := transport.NewFlipFlop(artifact, pipeline)

		So(err, ShouldBeNil)

		Convey("It should keep all class probabilities on the wire", func() {
			So(datura.Peek[float64](artifact, "output", "s1"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "s0"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "s2"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkSoftmax_Read(testingTB *testing.B) {
	softmax := probability.NewSoftmax(softmaxSchema("s0", "s1", "s2", "s3"))
	artifact := artifactWithScores(map[string]float64{
		"s0": 0.2,
		"s1": 0.4,
		"s2": 0.7,
		"s3": 0.1,
	})

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, softmax)
	}
}
