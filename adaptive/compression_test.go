package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var compressionInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestCompressionRead(t *testing.T) {
	Convey("Given a Compression", t, func() {
		compression := NewCompression()
		io.Copy(compression, compressionInput)

		Convey("When Read is called", func() {
			_, err := compression.Read([]byte{1, 2, 3})
			So(err, ShouldBeNil)
			So(datura.Peek[float64](compression.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestCompressionWrite(t *testing.T) {
	Convey("Given a Compression", t, func() {
		compression := NewCompression()

		Convey("When Write is called", func() {
			_, err := io.Copy(compression, compressionInput)
			So(err, ShouldBeNil)
		})
	})
}
