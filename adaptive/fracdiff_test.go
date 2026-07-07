package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFracDiff_Measure(testingTB *testing.T) {
	Convey("Given a capped fractional differencing filter", testingTB, func() {
		fractional := NewFracDiff(FracDiffConfig{MaxLag: 4})

		for _, sample := range []float64{10, 1000000, -1000000, 2500000, -2500000} {
			_, err := fractional.Measure(sample)
			So(err, ShouldBeNil)
		}

		Convey("It should keep history and weights inside the configured lag", func() {
			So(len(fractional.history), ShouldEqual, 5)
			So(cap(fractional.weights), ShouldEqual, 5)
			So(fractional.width, ShouldBeLessThanOrEqualTo, 5)
			So(fractional.count, ShouldEqual, 5)
		})
	})
}

func BenchmarkFracDiff_Measure(benchmark *testing.B) {
	fractional := NewFracDiff()
	samples := []float64{10, 1000000, -1000000, 2500000, -2500000}

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		for _, sample := range samples {
			if _, err := fractional.Measure(sample); err != nil {
				benchmark.Fatal(err)
			}
		}
	}
}
