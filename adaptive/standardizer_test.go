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
			So(out1.Precision, ShouldEqual, 0)

			out2, err2 := standardizer.Measure(12)
			So(err2, ShouldBeNil)
			So(out2.Value, ShouldEqual, 0)
			So(out2.Ready, ShouldBeFalse)
			So(out2.Precision, ShouldEqual, 0)
		})

		Convey("Then a feature is scoreable as soon as it has any spread", func() {
			standardizer.Standardize(10)
			standardizer.Standardize(12)

			out, err := standardizer.Measure(20)
			So(err, ShouldBeNil)
			So(out.Ready, ShouldBeTrue)
			So(out.Value, ShouldBeGreaterThan, 0)
		})

		Convey("Then an early score is shrunk by the precision of its own moments", func() {
			standardizer.Standardize(10)
			standardizer.Standardize(12)

			early, err := standardizer.Measure(14)
			So(err, ShouldBeNil)

			mature := NewStandardizer()

			for index := range 200 {
				mature.Standardize(10 + float64(index%3))
			}

			settled, err := mature.Measure(14)
			So(err, ShouldBeNil)

			So(early.Precision, ShouldBeLessThan, 0.5)
			So(early.Precision, ShouldBeLessThan, settled.Precision)
			So(settled.Precision, ShouldBeGreaterThan, 0.9)
			So(settled.Precision, ShouldBeLessThan, 1)
		})

		Convey("Then a settled feature converges on the exact prior z-score", func() {
			readings := make([]float64, 0, 4096)
			mean := 0.0

			for index := range 4096 {
				reading := float64(index % 97)
				readings = append(readings, reading)
				mean += reading
				standardizer.Standardize(reading)
			}

			mean /= float64(len(readings))
			variance := 0.0

			for _, reading := range readings {
				variance += math.Pow(reading-mean, 2)
			}

			variance /= float64(len(readings) - 1)
			reading := 50.0
			standardized := standardizer.Standardize(reading)

			expected := (reading - mean) / math.Sqrt(variance)
			So(standardized, ShouldAlmostEqual, expected, 1e-2)
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
