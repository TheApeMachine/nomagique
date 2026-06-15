package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMin_Observe(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"mixed batch", []float64{3, 1, 4, 2}, 1},
		{"negative floor", []float64{-5, -1, -3}, -5},
		{"single value", []float64{9}, 9},
		{"empty input", nil, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			minStage := NewMin()
			got := observeInputs(minStage, testCase.samples...)

			Convey("It should return the expected minimum", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestMin_Reset(testingTB *testing.T) {
	Convey("Given an observed min", testingTB, func() {
		minStage := NewMin()
		_ = observeInputs(minStage, 3, 1)

		So(minStage.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(observeInputs(minStage)), ShouldEqual, 0)
		})
	})
}

func BenchmarkMin_Observe(b *testing.B) {
	minStage := NewMin()

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(minStage)
	}
}
