package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFastSlowRate(testingTB *testing.T) {
	cases := []struct {
		name   string
		stream []float64
		invert bool
		expect func(value float64) bool
	}{
		{
			name:   "breakout stream",
			stream: []float64{0, 0, 0, 10, 10, 10},
			invert: false,
			expect: func(value float64) bool { return value > 1 },
		},
		{
			name:   "inverted compression stream",
			stream: []float64{0.5, 0.5, 0.5, 0.5, 0.2, 0.2, 0.2},
			invert: true,
			expect: func(value float64) bool { return value > 1 },
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			var ratio *FastSlow

			if testCase.invert {
				ratio = NewInvertedFastSlow(3, 1e-6)
			}

			if !testCase.invert {
				ratio = NewFastSlow(3, 1e-6)
			}

			got := observeInputs(ratio, testCase.stream...)

			Convey("It should return the expected ratio", func() {
				So(testCase.expect(float64(got)), ShouldBeTrue)
			})
		})
	}
}

func TestFastSlow_Reset(testingTB *testing.T) {
	Convey("Given an observed fast-slow stage", testingTB, func() {
		ratio := NewFastSlow(3, 1e-6)
		_ = observeInputs(ratio, 0, 0, 0, 10, 10, 10)

		So(ratio.Reset(), ShouldBeNil)

		Convey("It should clear stream and output", func() {
			So(float64(observeInputs(ratio, 0, 0, 0, 0, 0, 0)), ShouldEqual, 1)
		})
	})
}

func BenchmarkFastSlowRate(b *testing.B) {
	samples := make([]float64, 128)

	for index := range samples {
		samples[index] = float64(index%5) + 1
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = FastSlowRate(samples, 3, 1e-6)
	}
}

func BenchmarkFastSlow_Observe(b *testing.B) {
	stream := []float64{0, 0, 0, 0, 0, 10, 10, 10, 12, 12, 12}
	ratio := NewFastSlow(3, 1e-6)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(ratio, stream...)
	}
}
