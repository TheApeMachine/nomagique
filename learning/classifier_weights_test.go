package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func testClassifierConfig() *datura.Artifact {
	return datura.Acquire("classifier-weights-config", datura.APPJSON).
		Poke([]string{"ignition", "compression", "trend", "exhaustion"}, "outputs").
		Poke(map[string]any{
			"terms": []string{"rvol", "precursor"},
		}, "ignition").
		Poke(map[string]any{
			"terms":   []string{"compression", "precursor"},
			"inverts": []string{"precursor"},
		}, "compression").
		Poke(map[string]any{
			"terms":   []string{"precursor", "compression", "rvol"},
			"inverts": []string{"compression"},
		}, "trend").
		Poke(map[string]any{
			"terms":   []string{"rvol", "precursor"},
			"inverts": []string{"rvol", "precursor"},
		}, "exhaustion")
}

func testClassifierScales() map[string]float64 {
	return map[string]float64{
		"rvol":        2.0,
		"precursor":   0.1,
		"compression": 1.5,
	}
}

func TestClassifierWeightsScores(testingTB *testing.T) {
	Convey("Given configured output recipes", testingTB, func() {
		config := testClassifierConfig()
		weights, err := NewClassifierWeights(config, 2.0, testClassifierScales())
		So(err, ShouldBeNil)

		scores := weights.Scores(map[string]float64{
			"rvol":        2.0,
			"precursor":   0.1,
			"compression": 1.5,
		})

		Convey("It should produce configured logits", func() {
			So(len(scores), ShouldEqual, 4)
			So(scores[0], ShouldBeGreaterThan, 0)
		})
	})
}

func TestClassifierWeightsNegativeFeatures(testingTB *testing.T) {
	Convey("Given negative feature values", testingTB, func() {
		config := testClassifierConfig()
		weights, err := NewClassifierWeights(config, 2.0, testClassifierScales())
		So(err, ShouldBeNil)

		negativeScores := weights.Scores(map[string]float64{
			"rvol":        -2.0,
			"precursor":   -0.1,
			"compression": -1.5,
		})
		zeroScores := weights.Scores(map[string]float64{
			"rvol":        0,
			"precursor":   0,
			"compression": 0,
		})

		Convey("It should preserve negative contribution instead of squashing to zero", func() {
			So(negativeScores[0], ShouldBeLessThan, 0)
			So(negativeScores[0], ShouldNotEqual, zeroScores[0])
		})
	})
}

func TestNewClassifierWeightsInvalidScale(testingTB *testing.T) {
	Convey("Given a non-positive feature scale", testingTB, func() {
		config := testClassifierConfig()
		_, err := NewClassifierWeights(config, 2.0, map[string]float64{})

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}
