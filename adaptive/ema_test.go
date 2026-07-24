package adaptive

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEMA_Measure(t *testing.T) {
	Convey("Given an EMA calculator", t, func() {
		ema, err := NewEMA(EMAConfig{Period: 2, Smoothing: 2})
		So(err, ShouldBeNil)

		Convey("When samples are measured directly", func() {
			value, measureErr := ema.Measure(1, 2, 3)

			Convey("It should return a finite smoothed value", func() {
				So(measureErr, ShouldBeNil)
				So(value, ShouldBeGreaterThan, 0)
				So(math.IsNaN(value), ShouldBeFalse)
				So(math.IsInf(value, 0), ShouldBeFalse)
			})
		})
	})

	Convey("Given an EMA with retained state", t, func() {
		ema, err := NewEMA(EMAConfig{Period: 2, Smoothing: 2})
		So(err, ShouldBeNil)
		first, err := ema.Measure(1)
		So(err, ShouldBeNil)

		Convey("When a second sample is measured", func() {
			second, measureErr := ema.Measure(2)

			Convey("It should move from the first reading", func() {
				So(measureErr, ShouldBeNil)
				So(second, ShouldBeGreaterThan, first)
			})
		})
	})

	Convey("Given an empty sample set", t, func() {
		ema, err := NewEMA(EMAConfig{Period: 2, Smoothing: 2})
		So(err, ShouldBeNil)

		Convey("When it is measured", func() {
			_, measureErr := ema.Measure()

			Convey("It should return an error", func() {
				So(measureErr, ShouldNotBeNil)
			})
		})
	})

	Convey("Given missing EMA parameters", t, func() {
		_, err := NewEMA()

		Convey("It should reject construction", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkEMA_Measure(b *testing.B) {
	ema, err := NewEMA(EMAConfig{Period: 2, Smoothing: 2})

	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, measureErr := ema.Measure(1, 2, 3); measureErr != nil {
			b.Fatal(measureErr)
		}
	}
}
