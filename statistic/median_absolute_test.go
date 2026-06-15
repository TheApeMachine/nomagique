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
			medianAbsolute := NewMedianAbsolute(nil)
			got := observeInputs(medianAbsolute, testCase.samples...)

			Convey("It should return the median absolute value", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestMedianAbsolute_Reset(testingTB *testing.T) {
	Convey("Given an observed stage", testingTB, func() {
		medianAbsolute := NewMedianAbsolute(nil)
		_ = observeInputs(medianAbsolute, -1, 2)

		So(medianAbsolute.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(observeInputs(medianAbsolute)), ShouldEqual, 0)
		})
	})
}

func BenchmarkMedianAbsolute_Observe(b *testing.B) {
	medianAbsolute := NewMedianAbsolute(nil)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(medianAbsolute)
	}
}
