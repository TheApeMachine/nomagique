package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDelta_Measure(t *testing.T) {
	Convey("Given a delta tracker", t, func() {
		delta := NewDelta()

		Convey("When the first sample arrives", func() {
			first, err := delta.Measure(10)

			Convey("Then it emits the maximal normalized change immediately", func() {
				So(err, ShouldBeNil)
				So(first.Ready, ShouldBeTrue)
				So(first.Value, ShouldEqual, 1)
				So(first.Count, ShouldEqual, 1)
			})
		})

		Convey("When a second, distinct sample arrives", func() {
			_, err := delta.Measure(10)
			So(err, ShouldBeNil)

			second, err := delta.Measure(14)

			Convey("Then it reports the range-normalized change", func() {
				So(err, ShouldBeNil)
				So(second.Ready, ShouldBeTrue)
				So(second.Value, ShouldEqual, 1)
				So(second.Count, ShouldEqual, 2)
			})
		})

		Convey("When every sample so far is identical", func() {
			_, err := delta.Measure(10)
			So(err, ShouldBeNil)

			flat, err := delta.Measure(10)

			Convey("Then it reports not ready, since range-normalization is indeterminate", func() {
				So(err, ShouldBeNil)
				So(flat.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkDelta_Measure(b *testing.B) {
	delta := NewDelta()
	_, _ = delta.Measure(10)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = delta.Measure(14)
	}
}
