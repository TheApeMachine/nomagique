package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewSoftmax(testingTB *testing.T) {
	Convey("Given typed softmax config", testingTB, func() {
		softmax := NewSoftmax(SoftmaxConfig{
			Inputs: []string{"s0", "s1"},
		})

		Convey("It should return a usable calculator", func() {
			So(softmax, ShouldNotBeNil)
		})
	})
}

func TestSoftmaxMeasure(testingTB *testing.T) {
	Convey("Given four typed output scores", testingTB, func() {
		softmax := NewSoftmax(SoftmaxConfig{
			Inputs:    []string{"s0", "s1", "s2", "s3"},
			Normalize: true,
		})
		output, err := softmax.Measure(softmaxInput(
			CategoryScore{Category: "s0", Score: 0.2},
			CategoryScore{Category: "s1", Score: 0.1},
			CategoryScore{Category: "s2", Score: 0.9},
			CategoryScore{Category: "s3", Score: 0.05},
		))

		Convey("It should expose every class probability", func() {
			So(err, ShouldBeNil)
			So(len(output.Probabilities), ShouldEqual, 4)
			So(sumProbabilities(output.Values), ShouldAlmostEqual, 1.0, 1e-9)
			So(output.Probabilities[2].Score, ShouldBeGreaterThan, 0.25)
			So(output.Probabilities[0].Category, ShouldEqual, "s0")
		})
	})

	Convey("Given an empty score key in config inputs", testingTB, func() {
		softmax := NewSoftmax(SoftmaxConfig{Inputs: []string{"s0", ""}})
		_, err := softmax.Measure(softmaxInput(CategoryScore{Category: "s0", Score: 1}))

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given uniform scores", testingTB, func() {
		softmax := NewSoftmax(SoftmaxConfig{
			Inputs: []string{"s0", "s1", "s2", "s3"},
		})
		output, err := softmax.Measure(softmaxInput(
			CategoryScore{Category: "s0", Score: 3},
			CategoryScore{Category: "s1", Score: 3},
			CategoryScore{Category: "s2", Score: 3},
			CategoryScore{Category: "s3", Score: 3},
		))

		Convey("It should emit the uniform distribution", func() {
			So(err, ShouldBeNil)
			So(output.Probabilities[0].Score, ShouldAlmostEqual, 0.25, 1e-9)
			So(output.Probabilities[1].Score, ShouldAlmostEqual, 0.25, 1e-9)
		})
	})

	Convey("Given non-finite scores", testingTB, func() {
		softmax := NewSoftmax(SoftmaxConfig{
			Inputs: []string{"s0", "s1", "s2"},
		})
		_, err := softmax.Measure(softmaxInput(
			CategoryScore{Category: "s0", Score: 1},
			CategoryScore{Category: "s1", Score: math.NaN()},
			CategoryScore{Category: "s2", Score: 3},
		))

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given raw mode in typed config", testingTB, func() {
		softmax := NewSoftmax(SoftmaxConfig{
			Inputs: []string{"s0", "s1", "s2"},
		})
		output, err := softmax.Measure(softmaxInput(
			CategoryScore{Category: "s0", Score: 1},
			CategoryScore{Category: "s1", Score: 2},
			CategoryScore{Category: "s2", Score: 3},
		))

		Convey("It should rank the largest logit highest", func() {
			So(err, ShouldBeNil)
			So(ArgmaxIndex(output.Values), ShouldEqual, 2)
		})
	})

	Convey("Given duplicate score keys", testingTB, func() {
		softmax := NewSoftmax(SoftmaxConfig{Inputs: []string{"s0"}})
		_, err := softmax.Measure(softmaxInput(
			CategoryScore{Category: "s0", Score: 1},
			CategoryScore{Category: "s0", Score: 2},
		))

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkSoftmaxMeasure(testingTB *testing.B) {
	softmax := NewSoftmax(SoftmaxConfig{
		Inputs: []string{"s0", "s1", "s2", "s3"},
	})
	input := softmaxInput(
		CategoryScore{Category: "s0", Score: 0.2},
		CategoryScore{Category: "s1", Score: 0.4},
		CategoryScore{Category: "s2", Score: 0.7},
		CategoryScore{Category: "s3", Score: 0.1},
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = softmax.Measure(input)
	}
}

func softmaxInput(scores ...CategoryScore) SoftmaxInput {
	return SoftmaxInput{
		Scores: scores,
	}
}
