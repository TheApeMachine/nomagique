package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFastSlowRate(testingTB *testing.T) {
	cases := []struct {
		name     string
		samples  []float64
		window   int
		epsilon  float64
		expectFn func(float64) bool
	}{
		{
			name:    "silent slow window and recent spike",
			samples: []float64{0, 0, 0, 0, 0, 10, 10, 10},
			window:  3,
			epsilon: 1e-6,
			expectFn: func(rate float64) bool {
				return rate > 1.0
			},
		},
		{
			name:    "fewer samples than fast window",
			samples: []float64{1, 2},
			window:  3,
			epsilon: 1e-6,
			expectFn: func(rate float64) bool {
				return rate == 1.0
			},
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			rate := FastSlowRate(testCase.samples, testCase.window, testCase.epsilon)

			Convey("It should return the expected ratio", func() {
				So(testCase.expectFn(rate), ShouldBeTrue)
			})
		})
	}
}

func TestInvertedFastSlowRate(testingTB *testing.T) {
	Convey("Given tightening spreads", testingTB, func() {
		spreads := []float64{0.5, 0.5, 0.5, 0.5, 0.2, 0.2, 0.2}
		compression := InvertedFastSlowRate(spreads, 3, 1e-6)

		Convey("It should exceed unity", func() {
			So(compression, ShouldBeGreaterThan, 1.0)
		})
	})
}

func TestFastSlowRatio_Next(testingTB *testing.T) {
	Convey("Given a negative volume sample", testingTB, func() {
		ratio := NewFastSlowRatio(3, 1e-6)

		_, err := ratio.Next(0, -1.0)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a breakout sample series", testingTB, func() {
		ratio := NewFastSlowRatio(3, 1e-6)

		out, err := ratio.Next(0, []float64{1, 1, 1, 4, 4, 4}...)

		Convey("It should compute a breakout ratio without error", func() {
			So(err, ShouldBeNil)
			So(out, ShouldBeGreaterThan, 1.0)
		})
	})
}

func TestFastSlow_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		stream []float64
		invert bool
		expect func(float64) bool
	}{
		{
			name:   "breakout stream",
			stream: []float64{0, 0, 0, 0, 0, 10, 10, 10},
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
			var ratio *FastSlow[float64]

			if testCase.invert {
				ratio = NewInvertedFastSlow[float64](testCase.stream, 3, 1e-6)
			}

			if !testCase.invert {
				ratio = NewFastSlow[float64](testCase.stream, 3, 1e-6)
			}

			got := ratio.Observe()

			Convey("It should return the expected ratio", func() {
				So(testCase.expect(float64(got)), ShouldBeTrue)
			})
		})
	}
}

func TestFastSlow_Reset(testingTB *testing.T) {
	Convey("Given an observed fast-slow stage", testingTB, func() {
		ratio := NewFastSlow[float64]([]float64{0, 0, 0, 10, 10, 10}, 3, 1e-6)
		_ = ratio.Observe()

		So(ratio.Reset(), ShouldBeNil)

		Convey("It should clear stream and output", func() {
			So(ratio.stream, ShouldBeNil)
			So(float64(ratio.Observe()), ShouldEqual, 1)
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
	ratio := NewFastSlow[float64](stream, 3, 1e-6)

	b.ReportAllocs()

	for b.Loop() {
		_ = ratio.Observe()
	}
}
