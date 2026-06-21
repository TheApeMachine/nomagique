package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var rangeInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestRangeRead(t *testing.T) {
	Convey("Given a Range", t, func() {
		extent := NewRange(datura.Acquire("range-config", datura.APPJSON))
		io.Copy(extent, rangeInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := extent.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](extent.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestRangeWrite(t *testing.T) {
	Convey("Given a Range", t, func() {
		extent := NewRange(datura.Acquire("range-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(extent, rangeInput)
			So(err, ShouldBeNil)
		})
	})
}
