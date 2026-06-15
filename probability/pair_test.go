package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParsePredictedActual(testingTB *testing.T) {
	cases := []struct {
		name            string
		primary         float64
		extras          []float64
		expectPredicted float64
		expectActual    float64
		expectError     error
	}{
		{
			name:            "primary plus one extra",
			primary:         10,
			extras:          []float64{12},
			expectPredicted: 10,
			expectActual:    12,
		},
		{
			name:            "two extras ignore primary",
			primary:         0,
			extras:          []float64{10, 12},
			expectPredicted: 10,
			expectActual:    12,
		},
		{
			name:        "zero predicted in work pair",
			primary:     0,
			extras:      []float64{0, 10},
			expectError: ErrZeroPredicted,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			predicted, actual, err := parsePredictedActual(
				testCase.primary, testCase.extras,
			)

			if testCase.expectError != nil {
				Convey("It should return the expected error", func() {
					So(err, ShouldEqual, testCase.expectError)
				})

				return
			}

			Convey("It should parse predicted and actual", func() {
				So(err, ShouldBeNil)
				So(predicted, ShouldEqual, testCase.expectPredicted)
				So(actual, ShouldEqual, testCase.expectActual)
			})
		})
	}
}

func TestParseBernoulliOutcome(testingTB *testing.T) {
	cases := []struct {
		name        string
		primary     float64
		extras      []float64
		expect      float64
		expectError error
	}{
		{
			name:    "pair success",
			primary: 0,
			extras:  []float64{10, 12},
			expect:  1,
		},
		{
			name:    "raw probability",
			primary: 0.75,
			extras:  nil,
			expect:  0.75,
		},
		{
			name:        "invalid raw outcome",
			primary:     1.5,
			extras:      nil,
			expectError: ErrInvalidOutcome,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			outcome, err := parseBernoulliOutcome(testCase.primary, testCase.extras)

			if testCase.expectError != nil {
				Convey("It should return the expected error", func() {
					So(err, ShouldEqual, testCase.expectError)
				})

				return
			}

			Convey("It should return the expected outcome", func() {
				So(err, ShouldBeNil)
				So(outcome, ShouldEqual, testCase.expect)
			})
		})
	}
}
