package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCategoryShareConfidence(t *testing.T) {
	Convey("CategoryShareConfidence yields 1/N for equal evidence", t, func() {
		confidence, err := CategoryShareConfidence([]float64{1, 1, 1, 1, 1}, 1)

		So(err, ShouldBeNil)
		So(confidence, ShouldAlmostEqual, 0.2, 1e-9)
	})

	Convey("It stays near 1/N for close calls", t, func() {
		confidence, err := CategoryShareConfidence([]float64{0.21, 0.20, 0.20, 0.20, 0.19}, 1)

		So(err, ShouldBeNil)
		So(confidence, ShouldAlmostEqual, 0.2, 0.02)
	})

	Convey("It rises when one category dominates but never reaches 1", t, func() {
		closeCall, err := CategoryShareConfidence([]float64{0.21, 0.20, 0.20, 0.20, 0.19}, 1)
		dominant, err2 := CategoryShareConfidence([]float64{5, 1, 1, 1, 1}, 1)
		loneWinner, err3 := CategoryShareConfidence([]float64{0, 0, 0.6, 0}, 3)

		So(err, ShouldBeNil)
		So(err2, ShouldBeNil)
		So(err3, ShouldBeNil)
		So(dominant, ShouldBeGreaterThan, closeCall)
		So(dominant, ShouldAlmostEqual, 6.0/14.0, 1e-9)
		So(loneWinner, ShouldAlmostEqual, 1.6/4.6, 1e-9)
		So(loneWinner, ShouldBeLessThan, 1)
	})

	Convey("It returns zero when the selected category has no evidence", t, func() {
		confidence, err := CategoryShareConfidence([]float64{0.5, 0.3, 0.2, 0}, 4)

		So(err, ShouldBeNil)
		So(confidence, ShouldEqual, 0)
	})
}

func BenchmarkCategoryShareConfidence(testingTB *testing.B) {
	scores := []float64{0.6, 0.4, 0.7, 0.3}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = CategoryShareConfidence(scores, 1)
	}
}
