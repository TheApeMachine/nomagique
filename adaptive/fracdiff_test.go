package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFracDiff_Measure(testingTB *testing.T) {
	Convey("Given a capped fractional differencing filter", testingTB, func() {
		fractional, err := NewFracDiff(FracDiffConfig{
			MaxLag:          4,
			Order:           0.5,
			WeightThreshold: 1e-3,
		})
		So(err, ShouldBeNil)

		for _, sample := range []float64{10, 11, 12, 13, 14} {
			_, measureErr := fractional.Measure(sample)
			So(measureErr, ShouldBeNil)
		}

		Convey("It should keep history and weights inside the configured lag", func() {
			So(len(fractional.history), ShouldEqual, 5)
			So(fractional.width, ShouldBeLessThanOrEqualTo, 5)
			So(fractional.count, ShouldEqual, 5)
		})
	})
}

func BenchmarkFracDiff_Measure(benchmark *testing.B) {
	fractional, err := NewFracDiff(FracDiffConfig{
		MaxLag:          8,
		Order:           0.5,
		WeightThreshold: 1e-4,
	})

	if err != nil {
		benchmark.Fatal(err)
	}

	samples := []float64{10, 11, 12, 13, 14}

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		for _, sample := range samples {
			if _, measureErr := fractional.Measure(sample); measureErr != nil {
				benchmark.Fatal(measureErr)
			}
		}
	}
}
