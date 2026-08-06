package adaptive

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStandardizer(t *testing.T) {
	Convey("Given a feature standardizer with exact prior moments", t, func() {
		standardizer := NewStandardizer()

		Convey("Then the first two readings stay unscaled because no prior variance exists", func() {
			out1, err1 := standardizer.Measure(10)
			So(err1, ShouldBeNil)
			So(out1.Value, ShouldEqual, 0)
			So(out1.Ready, ShouldBeFalse)

			out2, err2 := standardizer.Measure(12)
			So(err2, ShouldBeNil)
			So(out2.Value, ShouldEqual, 0)
			So(out2.Ready, ShouldBeFalse)
		})

		Convey("Then a mature feature is standardized by the exact prior z-score", func() {
			warmup := defaultStandardizerWarmup
			readings := make([]float64, 0, warmup)
			mean := 0.0

			for index := range warmup {
				reading := float64(index)
				readings = append(readings, reading)
				mean += reading
				So(standardizer.Standardize(reading), ShouldEqual, 0)
			}

			mean /= float64(len(readings))
			variance := 0.0

			for _, reading := range readings {
				variance += math.Pow(reading-mean, 2)
			}

			variance /= float64(len(readings) - 1)
			reading := float64(warmup)
			standardized := standardizer.Standardize(reading)

			expected := (reading - mean) / math.Sqrt(variance)
			So(standardized, ShouldAlmostEqual, expected, 1e-6)
		})

		Convey("Then a feature with zero prior variance remains centered at zero", func() {
			standardizer.Standardize(5)
			standardizer.Standardize(5)

			So(standardizer.Standardize(5), ShouldEqual, 0)
		})

		Convey("Then extreme excursions are clamped to the sigma bound", func() {
			for index := range 50 {
				standardizer.Standardize(float64(index))
			}

			hugeReading := 1_000_000.0
			output, err := standardizer.Measure(hugeReading)

			So(err, ShouldBeNil)
			So(output.Value, ShouldEqual, defaultStandardizerSigmaBound)
		})

		Convey("Then non-finite values return validation errors", func() {
			_, err := standardizer.Measure(math.NaN())
			So(err, ShouldNotBeNil)

			_, errInf := standardizer.Measure(math.Inf(1))
			So(errInf, ShouldNotBeNil)
		})

		Convey("Then reset clears all state", func() {
			for index := range 50 {
				standardizer.Standardize(float64(index))
			}

			standardizer.Reset()
			So(standardizer.Count(), ShouldEqual, 0)

			out, err := standardizer.Measure(100)
			So(err, ShouldBeNil)
			So(out.Value, ShouldEqual, 0)
			So(out.Ready, ShouldBeFalse)
		})
	})
}

func BenchmarkStandardizer(b *testing.B) {
	standardizer := NewStandardizer()

	for b.Loop() {
		_ = standardizer.Standardize(1.234)
	}
}
