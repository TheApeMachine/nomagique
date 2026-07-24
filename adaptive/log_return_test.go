package adaptive

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLogReturn_Measure(t *testing.T) {
	Convey("Given a log-return tracker", t, func() {
		logReturn, err := NewLogReturn(LogReturnConfig{ReturnLag: 1, MaxSeries: 1})
		So(err, ShouldBeNil)

		Convey("When the first sample arrives", func() {
			first, measureErr := logReturn.Measure(LogReturnSample{Value: 100, At: time.Unix(1, 0)})

			Convey("Then it is not ready until the lag is populated", func() {
				So(measureErr, ShouldBeNil)
				So(first.Ready, ShouldBeFalse)
				So(first.Value, ShouldEqual, 0)
				So(first.Count, ShouldEqual, 1)
			})
		})

		Convey("When a stale sample arrives after a fresher one", func() {
			_, measureErr := logReturn.Measure(LogReturnSample{Value: 110, At: time.Unix(2, 0)})
			So(measureErr, ShouldBeNil)

			_, measureErr = logReturn.Measure(LogReturnSample{Value: 105, At: time.Unix(1, 0)})

			Convey("Then it reports timestamp regression", func() {
				So(measureErr, ShouldNotBeNil)
				So(measureErr.Error(), ShouldContainSubstring, "log-return: event timestamp must not regress")
			})
		})
	})
}

func BenchmarkLogReturn_Measure(b *testing.B) {
	logReturn, err := NewLogReturn(LogReturnConfig{ReturnLag: 1, MaxSeries: 1})

	if err != nil {
		b.Fatal(err)
	}

	_, _ = logReturn.Measure(LogReturnSample{Value: 100, At: time.Unix(1, 0)})

	b.ReportAllocs()

	for b.Loop() {
		_, _ = logReturn.Measure(LogReturnSample{
			Value: 110,
			At:    time.Unix(int64(b.N)+2, 0),
		})
	}
}
