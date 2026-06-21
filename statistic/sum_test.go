package statistic

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var sumInput = datura.Acquire("test", datura.APPJSON).Poke(1, "sample")

func TestSumRead(t *testing.T) {
	Convey("Given a Sum", t, func() {
		sum := NewSum(datura.Acquire("sum-config", datura.APPJSON))
		_, err := io.Copy(sum, sumInput)

		So(err, ShouldBeNil)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := sum.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](sum.artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}

func BenchmarkSumRead(b *testing.B) {
	sum := NewSum(datura.Acquire("sum-config-bench", datura.APPJSON))
	_, _ = io.Copy(sum, sumInput)
	frame := make([]byte, 65536)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = sum.Read(frame)
	}
}
