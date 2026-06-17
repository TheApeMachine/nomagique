package vector

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/tests"
)

func pipelineSample(stage io.ReadWriter, sample float64) float64 {
	value, _ := tests.PipelineSample([]io.ReadWriter{stage}, sample)

	return value
}

func TestNewFeatureNode(testingTB *testing.T) {
	Convey("Given a valid feature index", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		diffNode := NewFeatureNode(extractor, testDiffFeature)

		Convey("It should return a usable node", func() {
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
			node := NewFeatureNode(extractor, featureIndex)

			Convey("It should return nil", func() {
				So(node, ShouldBeNil)
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

			extractor.SetInput(testLeftChannel, testCase.left)
			extractor.SetInput(testRightChannel, testCase.right)

			featureNode := NewFeatureNode(extractor, testCase.featureIdx)

			got := observeInputs(featureNode)

			Convey("It should expose the derived feature", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given empty Observe after warm inputs", testingTB, func() {
		extractor := mustPairExtractor(testingTB)

		extractor.SetInput(testLeftChannel, 8)
		extractor.SetInput(testRightChannel, 2)

		diffNode := NewFeatureNode(extractor, testDiffFeature)

		Convey("It should refresh from extractor state", func() {
			So(float64(observeInputs(diffNode)), ShouldEqual, 6)
		})
	})

	Convey("Given a non-scalar input", testingTB, func() {
		extractor := mustPairExtractor(testingTB)

		extractor.SetInput(testLeftChannel, 10)
		extractor.SetInput(testRightChannel, 4)

		diffNode := NewFeatureNode(extractor, testDiffFeature)

		observeInputs(diffNode)

		Convey("It should still return the derived feature", func() {
			So(float64(observeWithoutSample(diffNode, 0)), ShouldEqual, 6)
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

			leftSlot := NewInputSlot(extractor, testLeftChannel)

			rightSlot := NewInputSlot(extractor, testRightChannel)

			sumNode := NewFeatureNode(extractor, testSumFeature)

			sampleSum := pipelineSample(leftSlot, testCase.left)
			scalarSum := pipelineSample(rightSlot, testCase.right)
			featureSum := float64(observeInputs(sumNode))

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

		extractor.SetInput(testLeftChannel, 10)
		extractor.SetInput(testRightChannel, 4)

		diffNode := NewFeatureNode(extractor, testDiffFeature)

		observeInputs(diffNode)

		Convey("When reset", func() {
			So(diffNode.Reset(), ShouldBeNil)

			Convey("It should succeed without clearing extractor state", func() {
				So(float64(observeInputs(diffNode)), ShouldEqual, 6)
			})
		})
	})
}

func TestFeatureNode_Number(testingTB *testing.T) {
	Convey("Given a composed product number", testingTB, func() {
		extractor := mustPairExtractor(testingTB)

		leftSlot := NewInputSlot(extractor, testLeftChannel)

		rightSlot := NewInputSlot(extractor, testRightChannel)

		productNode := NewFeatureNode(extractor, testProductFeature)

		observeInputs(leftSlot, 10)
		observeInputs(rightSlot, 3)

		pipeline := nomagique.Number(productNode)

		So(pipeline, ShouldNotBeNil)

		got := pipelineSample(pipeline, 0)

		Convey("It should observe through the registered pipeline", func() {
			So(got, ShouldEqual, 30)
		})
	})
}

func BenchmarkFeatureNode_Observe(b *testing.B) {
	extractor := mustPairExtractor(b)

	extractor.SetInput(testLeftChannel, 100)
	extractor.SetInput(testRightChannel, 3)

	diffNode := NewFeatureNode(extractor, testDiffFeature)
	b.ReportAllocs()

	for b.Loop() {
		observeInputs(diffNode)
	}
}
