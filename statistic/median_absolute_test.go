package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMedianAbsolute_Observe(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"signed spread", []float64{-3, 1, -2, 4}, 2.5},
		{"all negative", []float64{-5, -1, -3}, 3},
		{"empty input", nil, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			medianAbsolute := NewMedianAbsolute[float64](nil)
			got := medianAbsolute.Observe(numberInputs(testCase.samples...)...)

			Convey("It should return the median absolute value", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestMedianAbsolute_Reset(testingTB *testing.T) {
	Convey("Given an observed stage", testingTB, func() {
		medianAbsolute := NewMedianAbsolute[float64](nil)
		_ = medianAbsolute.Observe(numberInputs(-1, 2)...)

		So(medianAbsolute.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(medianAbsolute.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkMedianAbsolute_Observe(b *testing.B) {
	medianAbsolute := NewMedianAbsolute[float64](nil)
	inputs := numberInputs(-3, 1, -2, 4)

	b.ReportAllocs()

	for b.Loop() {
		_ = medianAbsolute.Observe(inputs...)
	}
}
