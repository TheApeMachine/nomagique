package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParseGrowthPair(testingTB *testing.T) {
	cases := []struct {
		name        string
		primary     float64
		extras      []float64
		expectLeft  float64
		expectRight float64
		expectError error
	}{
		{
			name:        "primary plus one extra",
			primary:     2,
			extras:      []float64{3},
			expectLeft:  2,
			expectRight: 3,
		},
		{
			name:        "two extras ignore primary",
			primary:     0,
			extras:      []float64{1.5, -0.5},
			expectLeft:  1.5,
			expectRight: -0.5,
		},
		{
			name:        "empty extras",
			primary:     1,
			extras:      nil,
			expectError: ErrEmptyInputs,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			left, right, err := parseGrowthPair(testCase.primary, testCase.extras)

			if testCase.expectError != nil {
				Convey("It should return the expected error", func() {
					So(err, ShouldEqual, testCase.expectError)
				})

				return
			}

			Convey("It should parse left and right growth", func() {
				So(err, ShouldBeNil)
				So(left, ShouldEqual, testCase.expectLeft)
				So(right, ShouldEqual, testCase.expectRight)
			})
		})
	}
}
