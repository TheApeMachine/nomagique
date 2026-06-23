package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var momentumInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestMomentumRead(t *testing.T) {
	Convey("Given a Momentum on the first sample", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON))
		_, _ = io.Copy(momentum, momentumInput)

		frame := make([]byte, 65536)
		_, err := momentum.Read(frame)

		So(err, ShouldNotBeNil)
	})

	Convey("Given a repeated span after bootstrap", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON))
		_, _ = io.Copy(momentum, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
		_, _ = momentum.Read(make([]byte, 65536))
		_, _ = io.Copy(momentum, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))

		_, err := momentum.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})

	Convey("Given warmed distinct samples", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON))
		_, _ = io.Copy(momentum, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
		_, _ = momentum.Read(make([]byte, 65536))
		_, _ = io.Copy(momentum, datura.Acquire("test", datura.APPJSON).Poke(14, "sample"))

		frame := make([]byte, 65536)
		readCount, err := momentum.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		outbound := datura.Acquire("momentum-outbound", datura.APPJSON)
		_, _ = outbound.Write(frame[:readCount])
		So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 1)
	})

	Convey("Given a non-finite sample", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON))
		invalid := datura.Acquire("test", datura.APPJSON).Poke(math.NaN(), "sample")
		_, _ = io.Copy(momentum, invalid)

		_, err := momentum.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
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
