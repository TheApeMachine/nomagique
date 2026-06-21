package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var momentumInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestMomentumRead(t *testing.T) {
	Convey("Given a Momentum", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON))
		io.Copy(momentum, momentumInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := momentum.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](momentum.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestMomentumWrite(t *testing.T) {
	Convey("Given a Momentum", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(momentum, momentumInput)
			So(err, ShouldBeNil)
		})
	})
}
