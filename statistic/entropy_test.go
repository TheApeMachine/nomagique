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
			entropy := NewEntropy(0)
			got := observeInputs(entropy, testCase.samples...)

			Convey("It should return the expected entropy", func() {
				So(testCase.compare(float64(got), testCase.expect), ShouldBeTrue)
			})
		})
	}

	Convey("Given a peaked distribution", testingTB, func() {
		uniform := observeInputs(NewEntropy(0), 1, 1, 1, 1)
		peaked := observeInputs(NewEntropy(0), 100, 1, 1, 1)

		Convey("It should be lower entropy than uniform", func() {
			So(float64(peaked), ShouldBeLessThan, float64(uniform))
		})
	})
}

func TestEntropy_Reset(testingTB *testing.T) {
	Convey("Given an observed entropy", testingTB, func() {
		entropy := NewEntropy(0)
		_ = observeInputs(entropy, 1, 1, 1, 1)

		So(entropy.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(observeInputs(entropy)), ShouldEqual, 0)
		})
	})
}

func BenchmarkEntropy_Observe(b *testing.B) {
	entropy := NewEntropy(0)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(entropy)
	}
}
