package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStdDev_Observe(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"uniform spread", []float64{1, 2, 3, 4, 5}, 1.5811388300841898},
		{"constant series", []float64{3, 3, 3}, 0},
		{"empty input", nil, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			stdDev := NewStdDev[float64](nil)
			got := stdDev.Observe(numberInputs(testCase.samples...)...)

			Convey("It should return the expected standard deviation", func() {
				So(float64(got), ShouldAlmostEqual, testCase.expect, 1e-12)
			})
		})
	}
}

func TestStdDev_Reset(testingTB *testing.T) {
	Convey("Given an observed stddev", testingTB, func() {
		stdDev := NewStdDev[float64]([]float64{1, 2})
		_ = stdDev.Observe(numberInputs(1, 2)...)

		So(stdDev.Reset(), ShouldBeNil)

		Convey("It should clear weights", func() {
			So(stdDev.weights, ShouldBeNil)
		})
	})
}

func BenchmarkStdDev_Observe(b *testing.B) {
	stdDev := NewStdDev[float64](nil)
	inputs := numberInputs(1, 2, 3, 4, 5)

	b.ReportAllocs()

	for b.Loop() {
		_ = stdDev.Observe(inputs...)
	}
}
