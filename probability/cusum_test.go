package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewCUSUM(testingTB *testing.T) {
	Convey("Given CUSUM constructor", testingTB, func() {
		changeSum := NewCUSUM()

		Convey("It should return a usable change detector", func() {
			So(changeSum, ShouldNotBeNil)
		})
	})
}

func TestCUSUMMeasure(testingTB *testing.T) {
	Convey("Given a first sample", testingTB, func() {
		changeSum := NewCUSUM()
		output, err := changeSum.Measure(10)

		Convey("It should emit zero change evidence", func() {
			So(err, ShouldBeNil)
			So(output.Value, ShouldEqual, 0)
			So(output.Ready, ShouldBeFalse)
			So(output.Count, ShouldEqual, 1)
		})
	})

	Convey("Given repeated equal samples", testingTB, func() {
		changeSum := NewCUSUM()
		var output CUSUMOutput
		var err error

		for _, sample := range []float64{10, 10} {
			output, err = changeSum.Measure(sample)

			So(err, ShouldBeNil)
		}

		Convey("It should advance count without manufacturing change evidence", func() {
			So(output.Count, ShouldEqual, 2)
			So(output.Positive, ShouldEqual, 0)
			So(output.Negative, ShouldEqual, 0)
			So(output.Value, ShouldEqual, 0)
		})
	})

	Convey("Given a warmed positive change sum", testingTB, func() {
		changeSum := NewCUSUM()
		_, _ = changeSum.Measure(10)
		output, err := changeSum.Measure(25)

		Convey("It should accumulate positive evidence", func() {
			So(err, ShouldBeNil)
			So(output.Value, ShouldBeGreaterThan, 0)
			So(output.Positive, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a warmed negative change sum", testingTB, func() {
		changeSum := NewCUSUM()
		_, _ = changeSum.Measure(10)
		output, err := changeSum.Measure(8)

		Convey("It should accumulate negative evidence", func() {
			So(err, ShouldBeNil)
			So(output.Value, ShouldBeGreaterThan, 0)
			So(output.Negative, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given invalid reference", testingTB, func() {
		changeSum := NewCUSUM(CUSUMConfig{Reference: -1})
		_, err := changeSum.Measure(10)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestCUSUMReset(testingTB *testing.T) {
	Convey("Given reset after observation", testingTB, func() {
		changeSum := NewCUSUM()
		_, _ = changeSum.Measure(10)
		_, _ = changeSum.Measure(25)

		changeSum.Reset()

		output, err := changeSum.Measure(10)

		Convey("It should clear derived state", func() {
			So(err, ShouldBeNil)
			So(output.Count, ShouldEqual, 1)
			So(output.Value, ShouldEqual, 0)
			So(output.Ready, ShouldBeFalse)
		})
	})
}

func BenchmarkCUSUMMeasure(testingTB *testing.B) {
	changeSum := NewCUSUM()
	_, _ = changeSum.Measure(10)
	_, _ = changeSum.Measure(10.5)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = changeSum.Measure(10.5)
	}
}
