package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var zscoreInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestZScoreRead(t *testing.T) {
	Convey("Given a ZScore", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON))
		io.Copy(surprise, zscoreInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := surprise.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](surprise.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestZScoreWrite(t *testing.T) {
	Convey("Given a ZScore", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(surprise, zscoreInput)
			So(err, ShouldBeNil)
		})
	})
}
