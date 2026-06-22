package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestDeltaRead(t *testing.T) {
	Convey("Given a Delta", t, func() {
		delta := NewDelta(datura.Acquire("delta-config", datura.APPJSON))

		Convey("When the first sample arrives", func() {
			_, _ = io.Copy(delta, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
			_, err := delta.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})

		Convey("When warmed up with distinct samples", func() {
			_, _ = io.Copy(delta, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
			_, _ = delta.Read(make([]byte, 65536))
			_, _ = io.Copy(delta, datura.Acquire("test", datura.APPJSON).Poke(22, "sample"))

			frame := make([]byte, 65536)
			readCount, err := delta.Read(frame)

			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](delta.artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestDeltaWrite(t *testing.T) {
	Convey("Given a Delta", t, func() {
		delta := NewDelta(datura.Acquire("delta-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(delta, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
			So(err, ShouldBeIn, nil, io.EOF)
		})
	})
}
