package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewMean(testingTB *testing.T) {
	Convey("Given NewMean", testingTB, func() {
		mean := NewMean[float64](nil)

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
			mean := NewMean[float64](nil)
			got := mean.Observe(numberInputs(testCase.samples...)...)

			Convey("It should return the expected mean", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given weighted samples", testingTB, func() {
		mean := NewMean[float64]([]float64{1, 1, 1, 3})
		got := mean.Observe(numberInputs(10, 10, 10, 30)...)

		Convey("It should apply weights", func() {
			So(float64(got), ShouldEqual, 20)
		})
	})
}

func TestMean_Reset(testingTB *testing.T) {
	Convey("Given an observed mean", testingTB, func() {
		mean := NewMean[float64]([]float64{1, 2})
		_ = mean.Observe(numberInputs(1, 2)...)

		So(mean.Reset(), ShouldBeNil)

		Convey("It should clear weights and output", func() {
			So(mean.weights, ShouldBeNil)
			So(float64(mean.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkMean_Observe(b *testing.B) {
	mean := NewMean[float64](nil)
	inputs := numberInputs(1, 2, 3, 4, 5)

	b.ReportAllocs()

	for b.Loop() {
		_ = mean.Observe(inputs...)
	}
}
