package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var rangeInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestRangeRead(t *testing.T) {
	Convey("Given a Range on the first sample", t, func() {
		extent := NewRange(datura.Acquire("range-config", datura.APPJSON))
		_, _ = io.Copy(extent, rangeInput)

		frame := make([]byte, 65536)
		_, err := extent.Read(frame)

		So(err, ShouldNotBeNil)
	})

	Convey("Given a repeated span after bootstrap", t, func() {
		extent := NewRange(datura.Acquire("range-config", datura.APPJSON))
		_, _ = io.Copy(extent, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
		_, _ = extent.Read(make([]byte, 65536))
		_, _ = io.Copy(extent, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))

		_, err := extent.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})

	Convey("Given warmed distinct samples", t, func() {
		extent := NewRange(datura.Acquire("range-config", datura.APPJSON))
		_, _ = io.Copy(extent, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
		_, _ = extent.Read(make([]byte, 65536))
		_, _ = io.Copy(extent, datura.Acquire("test", datura.APPJSON).Poke(14, "sample"))

		frame := make([]byte, 65536)
		readCount, err := extent.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		outbound := datura.Acquire("range-outbound", datura.APPJSON)
		_, _ = outbound.Write(frame[:readCount])
		So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 4)
	})

	Convey("Given a non-finite sample", t, func() {
		extent := NewRange(datura.Acquire("range-config", datura.APPJSON))
		invalid := datura.Acquire("test", datura.APPJSON).Poke(math.NaN(), "sample")
		_, _ = io.Copy(extent, invalid)

		_, err := extent.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
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
