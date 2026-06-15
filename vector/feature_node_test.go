package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewFeatureNode(testingTB *testing.T) {
	Convey("Given a valid feature index", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		diffNode, err := NewFeatureNode[float64](extractor, testDiffFeature)

		Convey("It should return a usable node", func() {
			So(err, ShouldBeNil)
			So(diffNode, ShouldNotBeNil)
		})
	})

	errorCases := []struct {
		name  string
		setup func() (*FeatureExtractor, int)
	}{
		{
			name: "nil extractor",
			setup: func() (*FeatureExtractor, int) {
				return nil, 0
			},
		},
		{
			name: "out of range feature",
			setup: func() (*FeatureExtractor, int) {
				extractor, err := newPairExtractor()
				So(err, ShouldBeNil)

				return extractor, 9
			},
		},
	}

	for _, testCase := range errorCases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			extractor, featureIndex := testCase.setup()
			_, err := NewFeatureNode[float64](extractor, featureIndex)

			Convey("It should return an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	}
}

func TestFeatureNode_Observe(testingTB *testing.T) {
	cases := []struct {
		name       string
		left       float64
		right      float64
		featureIdx int
		expect     float64
	}{
		{"sum feature", 10, 3, testSumFeature, 13},
		{"diff feature", 10, 3, testDiffFeature, 7},
		{"product feature", 10, 3, testProductFeature, 30},
		{"negative inputs", -4, 6, testSumFeature, 2},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			extractor, err := newPairExtractor()
			So(err, ShouldBeNil)

			_ = extractor.SetInput(testLeftChannel, testCase.left)
			_ = extractor.SetInput(testRightChannel, testCase.right)

			featureNode, err := NewFeatureNode[float64](extractor, testCase.featureIdx)
			So(err, ShouldBeNil)

			got := featureNode.Observe()

			Convey("It should expose the derived feature", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given empty Observe after warm inputs", testingTB, func() {
		extractor := mustPairExtractor(testingTB)

		_ = extractor.SetInput(testLeftChannel, 8)
		_ = extractor.SetInput(testRightChannel, 2)

		diffNode, err := NewFeatureNode[float64](extractor, testDiffFeature)
		So(err, ShouldBeNil)

		Convey("It should refresh from extractor state", func() {
			So(float64(diffNode.Observe()), ShouldEqual, 6)
		})
	})

	Convey("Given a non-scalar input", testingTB, func() {
		extractor := mustPairExtractor(testingTB)

		_ = extractor.SetInput(testLeftChannel, 10)
		_ = extractor.SetInput(testRightChannel, 4)

		diffNode, err := NewFeatureNode[float64](extractor, testDiffFeature)
		So(err, ShouldBeNil)

		_ = diffNode.Observe()
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should still return the derived feature", func() {
			So(float64(diffNode.Observe(stage)), ShouldEqual, 6)
		})
	})

	pathCases := []struct {
		name  string
		left  float64
		right float64
	}{
		{"monotone climb", 1, 2},
		{"volatile swing", 10, -3},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given slot updates for "+testCase.name, testingTB, func() {
			extractor := mustPairExtractor(testingTB)

			leftSlot, err := NewInputSlot[float64](extractor, testLeftChannel)
			So(err, ShouldBeNil)

			rightSlot, err := NewInputSlot[float64](extractor, testRightChannel)
			So(err, ShouldBeNil)

			sumNode, err := NewFeatureNode[float64](extractor, testSumFeature)
			So(err, ShouldBeNil)

			sampleSum := float64(
				core.Scalar[float64](testCase.left).Observe(leftSlot),
			)
			scalarSum := float64(
				core.Scalar[float64](testCase.right).Observe(rightSlot),
			)
			featureSum := float64(sumNode.Observe())

			Convey("It should derive the sum from updated channels", func() {
				So(sampleSum, ShouldEqual, testCase.left)
				So(scalarSum, ShouldEqual, testCase.right)
				So(featureSum, ShouldEqual, testCase.left+testCase.right)
			})
		})
	}
}

func TestFeatureNode_Reset(testingTB *testing.T) {
	Convey("Given an observed node", testingTB, func() {
		extractor := mustPairExtractor(testingTB)

		_ = extractor.SetInput(testLeftChannel, 10)
		_ = extractor.SetInput(testRightChannel, 4)

		diffNode, err := NewFeatureNode[float64](extractor, testDiffFeature)
		So(err, ShouldBeNil)

		_ = diffNode.Observe()

		Convey("When reset", func() {
			So(diffNode.Reset(), ShouldBeNil)

			Convey("It should succeed without clearing extractor state", func() {
				So(float64(diffNode.Observe()), ShouldEqual, 6)
			})
		})
	})
}

func TestFeatureNode_Number(testingTB *testing.T) {
	Convey("Given a composed product number", testingTB, func() {
		extractor := mustPairExtractor(testingTB)

		leftSlot, err := NewInputSlot[float64](extractor, testLeftChannel)
		So(err, ShouldBeNil)

		rightSlot, err := NewInputSlot[float64](extractor, testRightChannel)
		So(err, ShouldBeNil)

		productNode, err := NewFeatureNode[float64](extractor, testProductFeature)
		So(err, ShouldBeNil)

		_ = leftSlot.Observe(core.Scalar[float64](10))
		_ = rightSlot.Observe(core.Scalar[float64](3))

		product := nomagique.Number(productNode)

		Convey("It should observe through the registered pipeline", func() {
			So(product, ShouldEqual, 30)
		})
	})
}

func BenchmarkFeatureNode_Observe(b *testing.B) {
	extractor := mustPairExtractor(b)

	_ = extractor.SetInput(testLeftChannel, 100)
	_ = extractor.SetInput(testRightChannel, 3)

	diffNode, err := NewFeatureNode[float64](extractor, testDiffFeature)

	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = diffNode.Observe()
	}
}
