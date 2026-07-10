package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestVariance_Measure(t *testing.T) {
	Convey("Given a variance tracker", t, func() {
		variance := NewVariance()

		Convey("When the first sample arrives", func() {
			first, err := variance.Measure(10)

			Convey("Then it reports exactly zero variance around itself, ready", func() {
				So(err, ShouldBeNil)
				So(first.Ready, ShouldBeTrue)
				So(first.Value, ShouldEqual, 0)
				So(first.Count, ShouldEqual, 1)
			})
		})

		Convey("When every sample so far is identical", func() {
			_, err := variance.Measure(10)
			So(err, ShouldBeNil)

			flat, err := variance.Measure(10)

			Convey("Then it reports not ready, since the range is indeterminate", func() {
				So(err, ShouldBeNil)
				So(flat.Ready, ShouldBeFalse)
			})
		})

		Convey("When samples diverge", func() {
			_, err := variance.Measure(10)
			So(err, ShouldBeNil)

			second, err := variance.Measure(20)

			Convey("Then it reports a defined, ready variance", func() {
				So(err, ShouldBeNil)
				So(second.Ready, ShouldBeTrue)
				So(second.Count, ShouldEqual, 2)
			})
		})
	})
}

func BenchmarkVariance_Measure(b *testing.B) {
	variance := NewVariance()
	_, _ = variance.Measure(10)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = variance.Measure(14)
	}
}
