package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestClassifierWeightsScores(testingTB *testing.T) {
	Convey("Given default classifier weights", testingTB, func() {
		weights := DefaultClassifierWeights(2.0)
		scores := weights.Scores(2.0, 0.1, 1.5)

		Convey("It should produce four non-empty logits", func() {
			So(len(scores), ShouldEqual, 4)
			So(scores[0], ShouldBeGreaterThan, 0)
		})
	})
}
