package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var compressionInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

var compressionConfig = datura.Acquire("compression-config", datura.APPJSON)

func TestCompressionRead(t *testing.T) {
	Convey("Given a Compression", t, func() {
		compression := NewCompression(compressionConfig)
		io.Copy(compression, compressionInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 4096)
			_, err := compression.Read(frame)
			So(err, ShouldBeNil)

			outbound := datura.Acquire("test-out", datura.APPJSON)
			_, err = outbound.Write(frame)
			So(err, ShouldBeNil)

			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestCompressionWrite(t *testing.T) {
	Convey("Given a Compression", t, func() {
		compression := NewCompression(compressionConfig)

		Convey("When Write is called", func() {
			_, err := io.Copy(compression, compressionInput)
			So(err, ShouldBeNil)
		})
	})
}
