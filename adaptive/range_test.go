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
		extent := NewRange()
		io.Copy(extent, rangeInput)

		Convey("When Read is called", func() {
			_, err := extent.Read([]byte{1, 2, 3})
			So(err, ShouldBeNil)
			So(datura.Peek[float64](extent.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestRangeWrite(t *testing.T) {
	Convey("Given a Range", t, func() {
		extent := NewRange()

		Convey("When Write is called", func() {
			_, err := io.Copy(extent, rangeInput)
			So(err, ShouldBeNil)
		})
	})
}
