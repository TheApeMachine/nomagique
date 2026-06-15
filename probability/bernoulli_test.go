package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBernoulli(testingTB *testing.T) {
	Convey("Given Bernoulli constructor", testingTB, func() {
		posterior := NewBernoulli()

		Convey("It should return a usable dynamic", func() {
			So(posterior, ShouldNotBeNil)
		})
	})
}

func TestPosterior_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		inputs []float64
		expect func(float64) bool
	}{
		{
			name:   "unit success",
			inputs: []float64{1},
			expect: func(value float64) bool { return value > 0.5 },
		},
		{
			name:   "unit failure",
			inputs: []float64{0},
			expect: func(value float64) bool { return value < 0.5 },
		},
		{
			name:   "partial outcome",
			inputs: []float64{0.75},
			expect: func(value float64) bool { return value > 0.5 && value < 1 },
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			posterior := NewBernoulli()
			got := observeInputs(posterior, testCase.inputs...)

			Convey("It should return the expected posterior mean", func() {
				So(testCase.expect(got), ShouldBeTrue)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		posterior := NewBernoulli()

		Convey("It should return zero output", func() {
			So(observeInputs(posterior), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		posterior := NewBernoulli()
		before := observeInputs(posterior, 1)
		Convey("It should leave output unchanged", func() {
			So(observeWithoutSample(posterior, 99), ShouldEqual, before)
		})
	})

	Convey("Given a predicted and actual pair", testingTB, func() {
		posterior := NewBernoulli()
		_ = observeInputs(posterior, 10, 10)
		got := observeInputs(posterior, 10, 15)

		Convey("It should raise hit probability", func() {
			So(got, ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given an invalid outcome", testingTB, func() {
		posterior := NewBernoulli()
		got := observeInputs(posterior, 2)

		Convey("It should leave output at zero", func() {
			So(got, ShouldEqual, 0)
		})
	})
}

func TestPosterior_Reset(testingTB *testing.T) {
	Convey("Given an observed posterior", testingTB, func() {
		posterior := NewBernoulli()
		_ = observeInputs(posterior, 1)

		So(posterior.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(posterior.state.Ready, ShouldBeFalse)
			So(observeInputs(posterior), ShouldEqual, 0)
		})
	})
}

func BenchmarkBernoulli_Observe(testingTB *testing.B) {
	posterior := NewBernoulli()
	_ = observeInputs(posterior, 1)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(posterior, 10, 11)
	}
}
