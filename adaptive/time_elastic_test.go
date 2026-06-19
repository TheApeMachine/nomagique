package adaptive

import (
	"io"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestTimeElasticRead(t *testing.T) {
	Convey("Given a TimeElastic", t, func() {
		timeElastic := NewTimeElastic(time.Hour, 1e-6)
		input := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(float64(time.Unix(0, 1).UnixNano()), "at")
		io.Copy(timeElastic, input)

		Convey("When Read is called", func() {
			_, err := timeElastic.Read([]byte{1, 2, 3})
			So(err, ShouldBeNil)
			So(datura.Peek[float64](timeElastic.artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}

func TestTimeElasticWrite(t *testing.T) {
	Convey("Given a TimeElastic", t, func() {
		timeElastic := NewTimeElastic(time.Hour, 1e-6)

		Convey("When Write is called", func() {
			input := datura.Acquire("test", datura.APPJSON).
				Poke(10, "sample").
				Poke(float64(time.Unix(0, 1).UnixNano()), "at")
			_, err := io.Copy(timeElastic, input)
			So(err, ShouldBeNil)
		})
	})
}
