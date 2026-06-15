package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEntropy_Observe(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
		compare func(float64, float64) bool
	}{
		{
			name:    "uniform masses",
			samples: []float64{1, 1, 1, 1},
			expect:  math.Log(4),
			compare: func(got float64, expect float64) bool { return got == expect },
		},
		{
			name:    "empty input",
			samples: nil,
			expect:  0,
			compare: func(got float64, expect float64) bool { return got == expect },
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			entropy := NewEntropy[float64](0)
			got := entropy.Observe(numberInputs(testCase.samples...)...)

			Convey("It should return the expected entropy", func() {
				So(testCase.compare(float64(got), testCase.expect), ShouldBeTrue)
			})
		})
	}

	Convey("Given a peaked distribution", testingTB, func() {
		uniform := NewEntropy[float64](0).Observe(numberInputs(1, 1, 1, 1)...)
		peaked := NewEntropy[float64](0).Observe(numberInputs(100, 1, 1, 1)...)

		Convey("It should be lower entropy than uniform", func() {
			So(float64(peaked), ShouldBeLessThan, float64(uniform))
		})
	})
}

func TestEntropy_Reset(testingTB *testing.T) {
	Convey("Given an observed entropy", testingTB, func() {
		entropy := NewEntropy[float64](0)
		_ = entropy.Observe(numberInputs(1, 1, 1, 1)...)

		So(entropy.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(entropy.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkEntropy_Observe(b *testing.B) {
	entropy := NewEntropy[float64](0)
	inputs := numberInputs(1, 1, 1, 1, 2, 3)

	b.ReportAllocs()

	for b.Loop() {
		_ = entropy.Observe(inputs...)
	}
}
