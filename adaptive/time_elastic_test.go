package adaptive

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTimeElastic_Measure(t *testing.T) {
	Convey("Given a time-elastic baseline tracker", t, func() {
		timeElastic := NewTimeElastic(TimeElasticConfig{
			Halflife: time.Second,
			Epsilon:  0.01,
		})

		Convey("When the first sample establishes the baseline", func() {
			first, err := timeElastic.Measure(TimedValue{Value: 10, At: time.Unix(2, 0)})

			So(err, ShouldBeNil)
			So(first.Ready, ShouldBeTrue)
			So(first.Value, ShouldEqual, 1)
			So(first.Count, ShouldEqual, 1)
		})

		Convey("When a stale sample arrives after a fresher one", func() {
			_, err := timeElastic.Measure(TimedValue{Value: 10, At: time.Unix(2, 0)})
			So(err, ShouldBeNil)

			_, err = timeElastic.Measure(TimedValue{Value: 12, At: time.Unix(1, 0)})

			Convey("Then it reports timestamp regression", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "time-elastic: event timestamp must not regress")
			})
		})

		Convey("When a fresher sample arrives after the baseline", func() {
			_, err := timeElastic.Measure(TimedValue{Value: 10, At: time.Unix(1, 0)})
			So(err, ShouldBeNil)

			fresh, err := timeElastic.Measure(TimedValue{Value: 12, At: time.Unix(2, 0)})

			Convey("Then it emits a ready ratio", func() {
				So(err, ShouldBeNil)
				So(fresh.Ready, ShouldBeTrue)
				So(fresh.Value, ShouldBeGreaterThan, 1)
				So(fresh.Count, ShouldEqual, 2)
			})
		})
	})
}

func BenchmarkTimeElastic_Measure(b *testing.B) {
	timeElastic := NewTimeElastic(TimeElasticConfig{
		Halflife: time.Second,
		Epsilon:  0.01,
	})
	sample := TimedValue{Value: 12, At: time.Unix(2, 0)}

	_, _ = timeElastic.Measure(TimedValue{Value: 10, At: time.Unix(1, 0)})

	b.ReportAllocs()

	for b.Loop() {
		_, _ = timeElastic.Measure(sample)
	}
}
