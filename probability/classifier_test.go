package probability_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/probability"
)

func classifierSchema(categories ...string) probability.ClassifierSchema {
	return probability.ClassifierSchema{Categories: categories}
}

func classifierInput(scores map[string]float64, strength float64) probability.ClassifierInput {
	input := probability.ClassifierInput{
		Scores:   make([]probability.CategoryScore, 0, len(scores)),
		Strength: strength,
	}

	for category, score := range scores {
		input.Scores = append(input.Scores, probability.CategoryScore{
			Category: category,
			Score:    score,
		})
	}

	return input
}

func TestNewClassifier(testingTB *testing.T) {
	Convey("Given a classifier schema", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1"))

		Convey("It should construct a typed classifier", func() {
			So(classifier, ShouldNotBeNil)
		})
	})
}

func TestClassifier_Classify(testingTB *testing.T) {
	Convey("Given four category scores", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1", "s2", "s3"))

		result, err := classifier.Classify(classifierInput(
			map[string]float64{
				"s0": 0.2,
				"s1": 0.1,
				"s2": 0.9,
				"s3": 0.05,
			},
			0.9,
		))

		So(err, ShouldBeNil)

		Convey("It should return a 1-based winning category index", func() {
			So(result.Value, ShouldEqual, 3)
			So(result.Category, ShouldEqual, 3)
		})

		Convey("It should expose normalized probabilities", func() {
			So(result.Confidence, ShouldBeGreaterThan, 0)
			So(result.Confidence, ShouldBeLessThan, 1)
			So(result.Distribution["1"], ShouldAlmostEqual, result.Probabilities[0], 1e-12)
			So(result.Distribution["2"], ShouldAlmostEqual, result.Probabilities[1], 1e-12)
			So(result.Distribution["3"], ShouldAlmostEqual, result.Probabilities[2], 1e-12)
			So(result.Distribution["4"], ShouldAlmostEqual, result.Probabilities[3], 1e-12)
		})

		Convey("It should expose adaptive baselines from category competition", func() {
			So(result.Confidence, ShouldBeGreaterThan, result.ConfidenceBaseline)
			So(result.Confidence, ShouldBeGreaterThan, result.EntryBaseline)
			So(result.EntryBaseline, ShouldBeGreaterThanOrEqualTo, result.ExitBaseline)
		})
	})

	Convey("Given schema inputs are mutated after construction", testingTB, func() {
		categories := []string{"s0", "s1", "s2"}
		classifier := probability.NewClassifier(probability.ClassifierSchema{
			Categories: categories,
		})
		categories[1] = "mutated"

		result, err := classifier.Classify(classifierInput(
			map[string]float64{
				"s0": 0.1,
				"s1": 0.8,
				"s2": 0.2,
			},
			0.8,
		))

		Convey("It should classify using the construction-time category schema", func() {
			So(err, ShouldBeNil)
			So(result.Value, ShouldEqual, 2)
		})
	})

	Convey("Given an empty score key in schema inputs", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", ""))

		_, err := classifier.Classify(classifierInput(
			map[string]float64{"s0": 1},
			1,
		))

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given zero category evidence and zero strength", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1", "s2", "s3"))

		result, err := classifier.Classify(classifierInput(
			map[string]float64{
				"s0": 0,
				"s1": 0,
				"s2": 0,
				"s3": 0,
			},
			0,
		))

		So(err, ShouldBeNil)

		Convey("It should emit neutral confidence without positive edge", func() {
			So(result.Confidence, ShouldAlmostEqual, 0.25, 1e-9)
			So(result.ConfidenceBaseline, ShouldAlmostEqual, 0.25, 1e-12)
			So(result.EntryBaseline, ShouldAlmostEqual, 0.25, 1e-12)
			So(result.ExitBaseline, ShouldAlmostEqual, 0.25, 1e-12)
			So(result.Strength, ShouldEqual, 0)
		})
	})

	Convey("Given a duplicate score key", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1"))

		_, err := classifier.Classify(probability.ClassifierInput{
			Scores: []probability.CategoryScore{
				{Category: "s0", Score: 0.1},
				{Category: "s0", Score: 0.2},
				{Category: "s1", Score: 0.3},
			},
			Strength: 0.3,
		})

		Convey("It should reject ambiguous observations", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a missing category score", testingTB, func() {
		classifier := probability.NewClassifier(classifierSchema("s0", "s1"))

		_, err := classifier.Classify(classifierInput(
			map[string]float64{"s0": 0.1},
			0.1,
		))

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkClassifier_Classify(benchmark *testing.B) {
	classifier := probability.NewClassifier(classifierSchema("s0", "s1", "s2", "s3"))

	if classifier == nil {
		benchmark.Fatal("classifier required")
	}

	input := classifierInput(
		map[string]float64{
			"s0": 0.2,
			"s1": 0.4,
			"s2": 0.7,
			"s3": 0.1,
		},
		0.7,
	)

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		if _, err := classifier.Classify(input); err != nil {
			benchmark.Fatal(err)
		}
	}
}
