package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLinSpace(testingTB *testing.T) {
	cases := []struct {
		name   string
		start  float64
		end    float64
		count  int
		expect []float64
	}{
		{"three point span", 0, 1, 3, []float64{0, 0.5, 1}},
		{"single point", 2, 5, 1, []float64{2}},
		{"non-positive count", 0, 1, 0, nil},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			values := LinSpace(testCase.start, testCase.end, testCase.count)

			Convey("It should return evenly spaced values", func() {
				if testCase.expect == nil {
					So(values, ShouldBeNil)

					return
				}

				So(len(values), ShouldEqual, len(testCase.expect))

				for index, expect := range testCase.expect {
					So(values[index], ShouldEqual, expect)
				}
			})
		})
	}
}

func TestLogSpace(testingTB *testing.T) {
	cases := []struct {
		name      string
		start     float64
		end       float64
		count     int
		expectNil bool
	}{
		{"three point decade", 1, 10, 3, false},
		{"non-positive endpoint", 0, 10, 3, true},
		{"single point", 2, 8, 1, false},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			values := LogSpace(testCase.start, testCase.end, testCase.count)

			Convey("It should return logarithmically spaced values", func() {
				if testCase.expectNil {
					So(values, ShouldBeNil)

					return
				}

				So(len(values), ShouldEqual, testCase.count)
				So(values[0], ShouldEqual, testCase.start)
			})
		})
	}
}

func TestQuartiles(testingTB *testing.T) {
	cases := []struct {
		name  string
		input []float64
	}{
		{"full sample", []float64{1, 2, 3, 4, 5, 6, 7, 8}},
		{"single value", []float64{5}},
		{"empty input", nil},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			lower, upper := Quartiles(testCase.input)

			Convey("It should return ordered quartile bounds", func() {
				if len(testCase.input) == 0 {
					So(lower, ShouldEqual, 0)
					So(upper, ShouldEqual, 0)

					return
				}

				So(lower, ShouldBeLessThanOrEqualTo, upper)
			})
		})
	}
}

func BenchmarkLinSpace(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = LinSpace(0, 1, 128)
	}
}
