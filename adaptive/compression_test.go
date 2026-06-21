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
			readCount, err := compression.Read(frame)
			So(err, ShouldEqual, io.EOF)

			outbound := datura.Acquire("test-out", datura.APPJSON)
			_, err = outbound.Write(frame[:readCount])
			So(err, ShouldBeNil)

			rootKey := datura.Peek[string](outbound, "root")

			So(rootKey, ShouldEqual, "output")
			So(datura.Peek[float64](outbound, rootKey, "value"), ShouldEqual, 0)
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
