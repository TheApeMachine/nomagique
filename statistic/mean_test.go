package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewMean(testingTB *testing.T) {
	Convey("Given NewMean", testingTB, func() {
		mean := NewMean(nil)

		Convey("It should return a usable stage", func() {
			So(mean, ShouldNotBeNil)
		})
	})
}

func TestMean_Observe(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"uniform batch", []float64{1, 2, 3, 4}, 2.5},
		{"negative mix", []float64{-4, 8}, 2},
		{"single value", []float64{5}, 5},
		{"empty input", nil, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			mean := NewMean(nil)
			got := observeInputs(mean, testCase.samples...)

			Convey("It should return the expected mean", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given weighted samples", testingTB, func() {
		mean := NewMean([]float64{1, 1, 1, 3})
		got := observeInputs(mean, 10, 10, 10, 30)

		Convey("It should apply weights", func() {
			So(float64(got), ShouldEqual, 20)
		})
	})
}

func TestMean_Reset(testingTB *testing.T) {
	Convey("Given an observed mean", testingTB, func() {
		mean := NewMean([]float64{1, 2})
		_ = observeInputs(mean, 1, 2)

		So(mean.Reset(), ShouldBeNil)

		Convey("It should clear weights and output", func() {
			So(mean.weights, ShouldBeNil)
			So(float64(observeInputs(mean)), ShouldEqual, 0)
		})
	})
}

func BenchmarkMean_Observe(b *testing.B) {
	mean := NewMean(nil)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(mean)
	}
}
