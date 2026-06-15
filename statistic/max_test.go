package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMax_Observe(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"mixed batch", []float64{3, 1, 4, 2}, 4},
		{"negative ceiling", []float64{-5, -1, -3}, -1},
		{"single value", []float64{9}, 9},
		{"empty input", nil, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			maxStage := NewMax()
			got := observeInputs(maxStage, testCase.samples...)

			Convey("It should return the expected maximum", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestMax_Reset(testingTB *testing.T) {
	Convey("Given an observed max", testingTB, func() {
		maxStage := NewMax()
		_ = observeInputs(maxStage, 3, 1)

		So(maxStage.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(observeInputs(maxStage)), ShouldEqual, 0)
		})
	})
}

func BenchmarkMax_Observe(b *testing.B) {
	maxStage := NewMax()

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(maxStage)
	}
}
