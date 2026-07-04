package adaptive

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEMA_Measure(t *testing.T) {
	Convey("Given an EMA calculator", t, func() {
		ema := NewEMA(EMAConfig{Period: 2, Smoothing: 2})

		Convey("When samples are measured directly", func() {
			value, err := ema.Measure(1, 2, 3)

			Convey("It should return a finite smoothed value", func() {
				So(err, ShouldBeNil)
				So(value, ShouldBeGreaterThan, 0)
				So(math.IsNaN(value), ShouldBeFalse)
				So(math.IsInf(value, 0), ShouldBeFalse)
			})
		})
	})

	Convey("Given an EMA with retained state", t, func() {
		ema := NewEMA(EMAConfig{Period: 2, Smoothing: 2})
		first, err := ema.Measure(1)
		So(err, ShouldBeNil)

		Convey("When a second sample is measured", func() {
			second, err := ema.Measure(2)

			Convey("It should move from the first reading", func() {
				So(err, ShouldBeNil)
				So(second, ShouldBeGreaterThan, first)
			})
		})
	})

	Convey("Given an empty sample set", t, func() {
		ema := NewEMA()

		Convey("When it is measured", func() {
			_, err := ema.Measure()

			Convey("It should return an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func BenchmarkEMA_Measure(b *testing.B) {
	ema := NewEMA(EMAConfig{Period: 2, Smoothing: 2})

	b.ReportAllocs()

	for b.Loop() {
		if _, err := ema.Measure(1, 2, 3); err != nil {
			b.Fatal(err)
		}
	}
}
