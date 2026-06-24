package statistic

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestSumRead(t *testing.T) {
	Convey("Given a Sum", t, func() {
		config := datura.Acquire("sum-config", datura.APPJSON).Poke("sample", "input")
		sum := NewSum(config)
		input := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)
		_, err := io.Copy(sum, input)

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
	config := datura.Acquire("sum-config-bench", datura.APPJSON).Poke("sample", "input")
	sum := NewSum(config)
	input := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)
	_, _ = io.Copy(sum, input)
	frame := make([]byte, 65536)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = sum.Read(frame)
	}
}
