package adaptive

import (
	"io"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var timeElasticConfig = datura.Acquire("time-elastic-config", datura.APPJSON).
	Poke(float64(time.Hour), "config", "halflife").
	Poke(1e-6, "config", "epsilon")

func TestTimeElasticRead(t *testing.T) {
	Convey("Given a TimeElastic", t, func() {
		timeElastic := NewTimeElastic(timeElasticConfig)
		input := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(float64(time.Unix(0, 1).UnixNano()), "at")
		io.Copy(timeElastic, input)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := timeElastic.Read(frame)
			So(err, ShouldEqual, io.EOF)

			outbound := datura.Acquire("test-out", datura.APPJSON)
			_, err = outbound.Write(frame[:readCount])
			So(err, ShouldBeNil)

			rootKey := datura.Peek[string](outbound, "root")

			So(rootKey, ShouldEqual, "output")
			So(datura.Peek[float64](outbound, rootKey, "value"), ShouldEqual, 1)
		})
	})
}

func TestTimeElasticWrite(t *testing.T) {
	Convey("Given a TimeElastic", t, func() {
		timeElastic := NewTimeElastic(datura.Acquire("time-elastic-config", datura.APPJSON).
			Poke(float64(time.Hour), "config", "halflife").
			Poke(1e-6, "config", "epsilon"))

		Convey("When Write is called", func() {
			input := datura.Acquire("test", datura.APPJSON).
				Poke(10, "sample").
				Poke(float64(time.Unix(0, 1).UnixNano()), "at")
			_, err := io.Copy(timeElastic, input)
			So(err, ShouldBeNil)
		})
	})
}
