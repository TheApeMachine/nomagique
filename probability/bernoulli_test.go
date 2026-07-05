package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBernoulli(testingTB *testing.T) {
	Convey("Given Bernoulli constructor", testingTB, func() {
		posterior := NewBernoulli()

		Convey("It should return a usable posterior", func() {
			So(posterior, ShouldNotBeNil)
		})
	})
}

func TestBernoulliMeasure(testingTB *testing.T) {
	cases := []struct {
		name   string
		input  float64
		expect func(float64) bool
	}{
		{
			name:   "unit success",
			input:  1,
			expect: func(value float64) bool { return value > 0.5 },
		},
		{
			name:   "unit failure",
			input:  0,
			expect: func(value float64) bool { return value < 0.5 },
		},
		{
			name:   "partial outcome",
			input:  0.75,
			expect: func(value float64) bool { return value > 0.5 && value < 1 },
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			posterior := NewBernoulli()
			output, err := posterior.Measure(testCase.input)

			Convey("It should return the expected posterior mean", func() {
				So(err, ShouldBeNil)
				So(testCase.expect(output.Value), ShouldBeTrue)
				So(output.Count, ShouldEqual, 1)
			})
		})
	}

	Convey("Given a predicted and actual pair", testingTB, func() {
		posterior := NewBernoulli()
		output, err := posterior.MeasurePair(BernoulliPair{
			Predicted: 10,
			Actual:    15,
		})

		Convey("It should raise hit probability", func() {
			So(err, ShouldBeNil)
			So(output.Value, ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given repeated equal outcomes", testingTB, func() {
		posterior := NewBernoulli()

		for _, sample := range []float64{1, 1} {
			output, err := posterior.Measure(sample)

			So(err, ShouldBeNil)
			So(output.Count, ShouldBeGreaterThan, 0)
		}

		output, err := posterior.Measure(1)

		Convey("It should update the Beta posterior once per observation", func() {
			So(err, ShouldBeNil)
			So(output.Alpha, ShouldEqual, 4)
			So(output.Beta, ShouldEqual, 1)
			So(output.Count, ShouldEqual, 3)
			So(output.Value, ShouldEqual, 0.8)
		})
	})

	Convey("Given an invalid outcome", testingTB, func() {
		posterior := NewBernoulli()
		_, err := posterior.Measure(2)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestBernoulliReset(testingTB *testing.T) {
	Convey("Given an observed posterior", testingTB, func() {
		posterior := NewBernoulli()
		_, err := posterior.Measure(1)

		So(err, ShouldBeNil)

		posterior.Reset()
		output, err := posterior.Measure(0)

		Convey("It should clear derived state", func() {
			So(err, ShouldBeNil)
			So(output.Count, ShouldEqual, 1)
			So(output.Alpha, ShouldEqual, 1)
			So(output.Beta, ShouldEqual, 2)
		})
	})
}

func BenchmarkBernoulliMeasure(testingTB *testing.B) {
	posterior := NewBernoulli()
	_, _ = posterior.Measure(1)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = posterior.MeasurePair(BernoulliPair{
			Predicted: 10,
			Actual:    11,
		})
	}
}
