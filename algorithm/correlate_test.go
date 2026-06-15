package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCorrelate_Observe(testingTB *testing.T) {
	Convey("Given positively coupled sync and async streams", testingTB, func() {
		correlate := NewCorrelate(
			[]float64{1, 2, 3, 4, 5, 6},
			[]float64{2, 4, 6, 8, 10, 12},
			[]float64{0, 100, 1, 110, 2, 121, 3, 133.1},
			[]float64{0, 50, 1, 55, 2, 60.5, 3, 66.55},
			nil,
			time.Second,
		)

		pearson := correlate.Pearson()
		hayashi := correlate.Hayashi()
		gap := observeInputs(correlate)

		Convey("It should report positive synchronous correlation", func() {
			So(float64(pearson), ShouldBeGreaterThan, 0.9)
		})

		Convey("It should expose a finite async-sync gap", func() {
			So(float64(hayashi), ShouldAlmostEqual, 1, 1e-6)
			So(float64(gap), ShouldEqual, float64(hayashi)-float64(pearson))
		})
	})
}

func BenchmarkCorrelate_Observe(testingTB *testing.B) {
	correlate := NewCorrelate(
		[]float64{1, 2, 3, 4, 5, 6, 7, 8},
		[]float64{2, 4, 6, 8, 10, 12, 14, 16},
		[]float64{0, 100, 1, 110, 2, 120, 3, 130, 4, 140, 5, 150, 6, 160, 7, 170},
		[]float64{0, 100, 1, 120, 2, 140, 3, 160, 4, 180, 5, 200, 6, 220, 7, 240},
		nil,
		time.Second,
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(correlate)
	}
}
