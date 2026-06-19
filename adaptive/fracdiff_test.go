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
		fractional := NewFracDiff()
		io.Copy(fractional, fracDiffInput)

		Convey("When Read is called", func() {
			_, err := fractional.Read([]byte{1, 2, 3})
			So(err, ShouldBeNil)
			So(datura.Peek[float64](fractional.artifact, "output", "value"), ShouldEqual, 10)
		})
	})
}

func TestFracDiffWrite(t *testing.T) {
	Convey("Given a FracDiff", t, func() {
		fractional := NewFracDiff()

		Convey("When Write is called", func() {
			_, err := io.Copy(fractional, fracDiffInput)
			So(err, ShouldBeNil)
		})
	})
}
