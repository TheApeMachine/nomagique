package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var deltaInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

var deltaConfig = datura.Acquire("delta-config", datura.APPJSON)

func TestDeltaRead(t *testing.T) {
	Convey("Given a Delta", t, func() {
		delta := NewDelta(deltaConfig)
		io.Copy(delta, deltaInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := delta.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](delta.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestDeltaWrite(t *testing.T) {
	Convey("Given a Delta", t, func() {
		delta := NewDelta(datura.Acquire("delta-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(delta, deltaInput)
			So(err, ShouldBeNil)
		})
	})
}
