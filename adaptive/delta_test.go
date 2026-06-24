package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestDeltaRead(t *testing.T) {
	Convey("Given a Delta", t, func() {
		delta := NewDelta(datura.Acquire("delta-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When the first sample arrives", func() {
			_, _ = io.Copy(delta, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			frame := make([]byte, 65536)
			readCount, err := delta.Read(frame)

			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)

			outbound := datura.Acquire("delta-outbound", datura.APPJSON)
			_, _ = outbound.Write(frame[:readCount])
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
		})

		Convey("When warmed up with distinct samples", func() {
			_, _ = io.Copy(delta, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			_, _ = delta.Read(make([]byte, 65536))
			artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 22)

			err := transport.NewFlipFlop(artifact, delta)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestDeltaWrite(t *testing.T) {
	Convey("Given a Delta", t, func() {
		delta := NewDelta(datura.Acquire("delta-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When Write is called", func() {
			_, err := io.Copy(delta, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			So(err, ShouldBeIn, nil, io.EOF)
		})
	})
}
