package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewFeatureExtractor(testingTB *testing.T) {
	Convey("Given valid construction arguments", testingTB, func() {
		extractor, err := newPairExtractor()

		Convey("It should return a usable extractor", func() {
			So(err, ShouldBeNil)
			So(extractor, ShouldNotBeNil)
			So(extractor.InputCount(), ShouldEqual, 2)
			So(extractor.FeatureCount(), ShouldEqual, 3)
		})
	})

	errorCases := []struct {
		name        string
		inputCount  int
		formula     FeatureFormula
		expectError bool
	}{
		{"zero input count", 0, func([]float64) float64 { return 0 }, true},
		{"negative input count", -1, func([]float64) float64 { return 0 }, true},
		{"no formulas", 2, nil, true},
	}

	for _, testCase := range errorCases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			var formulas []FeatureFormula

			if testCase.formula != nil {
				formulas = []FeatureFormula{testCase.formula}
			}

			_, err := NewFeatureExtractor(testCase.inputCount, formulas...)

			Convey("It should return an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	}
}

func TestFeatureExtractor_SetInput(testingTB *testing.T) {
	cases := []struct {
		name      string
		left      float64
		right     float64
		expectSum float64
	}{
		{"positive pair", 10, 3, 13},
		{"negative left", -4, 6, 2},
		{"zero channels", 0, 0, 0},
		{"large magnitude", 1e9, 1e9, 2e9},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			extractor, err := newPairExtractor()
			So(err, ShouldBeNil)

			So(extractor.SetInput(testLeftChannel, testCase.left), ShouldBeNil)
			So(extractor.SetInput(testRightChannel, testCase.right), ShouldBeNil)

			features := extractor.Extract()

			Convey("It should derive the expected sum feature", func() {
				So(features[testSumFeature], ShouldEqual, testCase.expectSum)
			})
		})
	}

	Convey("Given an out-of-range channel", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		Convey("It should return an error", func() {
			So(extractor.SetInput(2, 1), ShouldNotBeNil)
			So(extractor.SetInput(-1, 1), ShouldNotBeNil)
		})
	})
}

func TestFeatureExtractor_Input(testingTB *testing.T) {
	Convey("Given a populated channel", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)
		So(extractor.SetInput(testLeftChannel, 7), ShouldBeNil)

		left, inputErr := extractor.Input(testLeftChannel)

		Convey("It should return the stored value", func() {
			So(inputErr, ShouldBeNil)
			So(left, ShouldEqual, 7)
		})
	})

	Convey("Given an out-of-range channel", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		_, inputErr := extractor.Input(9)

		Convey("It should return an error", func() {
			So(inputErr, ShouldNotBeNil)
		})
	})
}

func TestFeatureExtractor_Extract(testingTB *testing.T) {
	cases := []struct {
		name          string
		left          float64
		right         float64
		expectSum     float64
		expectDiff    float64
		expectProduct float64
	}{
		{"positive pair", 10, 3, 13, 7, 30},
		{"negative mix", -4, 6, 2, -10, -24},
		{"equal channels", 5, 5, 10, 0, 25},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			extractor, err := newPairExtractor()
			So(err, ShouldBeNil)

			So(extractor.SetInput(testLeftChannel, testCase.left), ShouldBeNil)
			So(extractor.SetInput(testRightChannel, testCase.right), ShouldBeNil)

			features := extractor.Extract()

			Convey("It should derive all registered features", func() {
				So(features[testSumFeature], ShouldEqual, testCase.expectSum)
				So(features[testDiffFeature], ShouldEqual, testCase.expectDiff)
				So(features[testProductFeature], ShouldEqual, testCase.expectProduct)
			})
		})
	}
}

func TestFeatureExtractor_Feature(testingTB *testing.T) {
	Convey("Given a populated extractor", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		_ = extractor.SetInput(testLeftChannel, 8)
		_ = extractor.SetInput(testRightChannel, 2)
		extractor.Extract()

		diff, featureErr := extractor.Feature(testDiffFeature)

		Convey("It should read one derived feature", func() {
			So(featureErr, ShouldBeNil)
			So(diff, ShouldEqual, 6)
		})
	})

	Convey("Given an out-of-range feature index", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		_, featureErr := extractor.Feature(9)

		Convey("It should return an error", func() {
			So(featureErr, ShouldNotBeNil)
		})
	})
}

func TestFeatureExtractor_Reset(testingTB *testing.T) {
	Convey("Given a populated extractor", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		_ = extractor.SetInput(testLeftChannel, 100)
		_ = extractor.SetInput(testRightChannel, 50)
		extractor.Extract()

		So(extractor.Reset(), ShouldBeNil)

		left, inputErr := extractor.Input(testLeftChannel)
		sum, featureErr := extractor.Feature(testSumFeature)

		Convey("It should clear inputs and features", func() {
			So(inputErr, ShouldBeNil)
			So(featureErr, ShouldBeNil)
			So(left, ShouldEqual, 0)
			So(sum, ShouldEqual, 0)
		})
	})
}

func BenchmarkFeatureExtractor_Extract(b *testing.B) {
	extractor, err := newPairExtractor()

	if err != nil {
		b.Fatal(err)
	}

	_ = extractor.SetInput(testLeftChannel, 100)
	_ = extractor.SetInput(testRightChannel, 3)

	b.ReportAllocs()

	for b.Loop() {
		extractor.Extract()
	}
}
