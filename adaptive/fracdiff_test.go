package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var fracDiffInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestFracDiffRead(t *testing.T) {
	Convey("Given a FracDiff", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON))
		io.Copy(fractional, fracDiffInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := fractional.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](fractional.artifact, "output", "value"), ShouldEqual, 10)
		})
	})
}

func TestFracDiffWrite(t *testing.T) {
	Convey("Given a FracDiff", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(fractional, fracDiffInput)
			So(err, ShouldBeNil)
		})
	})
}
