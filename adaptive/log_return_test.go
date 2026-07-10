package adaptive

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLogReturn_Measure(t *testing.T) {
	Convey("Given a log-return tracker", t, func() {
		logReturn := NewLogReturn()

		Convey("When the first sample arrives", func() {
			first, err := logReturn.Measure(LogReturnSample{Value: 100, At: time.Unix(1, 0)})

			Convey("Then it reports a defined zero return, ready", func() {
				So(err, ShouldBeNil)
				So(first.Ready, ShouldBeTrue)
				So(first.Value, ShouldEqual, 0)
				So(first.Count, ShouldEqual, 1)
			})
		})

		Convey("When a stale sample arrives after a fresher one", func() {
			_, err := logReturn.Measure(LogReturnSample{Value: 110, At: time.Unix(2, 0)})
			So(err, ShouldBeNil)

			_, err = logReturn.Measure(LogReturnSample{Value: 105, At: time.Unix(1, 0)})

			Convey("Then it reports timestamp regression", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "log-return: event timestamp must not regress")
			})
		})
	})
}
