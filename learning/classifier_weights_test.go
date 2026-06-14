package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func testClassifierScales() ClassifierFeatureScales {
	return ClassifierFeatureScales{
		RVol:        2.0,
		Precursor:   0.1,
		Compression: 1.5,
	}
}

func TestClassifierWeightsScores(testingTB *testing.T) {
	Convey("Given derived classifier weights", testingTB, func() {
		weights, err := NewClassifierWeights(2.0, testClassifierScales())
		So(err, ShouldBeNil)

		scores := weights.Scores(2.0, 0.1, 1.5)

		Convey("It should produce four non-empty logits", func() {
			So(len(scores), ShouldEqual, 4)
			So(scores[0], ShouldBeGreaterThan, 0)
		})
	})
}

func TestNewClassifierWeightsInvalidScale(testingTB *testing.T) {
	Convey("Given a non-positive feature scale", testingTB, func() {
		_, err := NewClassifierWeights(2.0, ClassifierFeatureScales{})

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}
