package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestVarianceRead(t *testing.T) {
	Convey("Given a Variance", t, func() {
		variance := NewVariance(datura.Acquire("variance-config", datura.APPJSON))

		Convey("When the first sample arrives", func() {
			_, err := io.Copy(variance, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))

			So(err, ShouldBeIn, nil, io.EOF)

			_, err = variance.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})

		Convey("When warmed up with distinct samples", func() {
			_, _ = io.Copy(variance, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
			_, _ = variance.Read(make([]byte, 65536))
			_, _ = io.Copy(variance, datura.Acquire("test", datura.APPJSON).Poke(22, "sample"))
			_, _ = variance.Read(make([]byte, 65536))
			_, _ = io.Copy(variance, datura.Acquire("test", datura.APPJSON).Poke(30, "sample"))
			_, _ = variance.Read(make([]byte, 65536))
			_, _ = io.Copy(variance, datura.Acquire("test", datura.APPJSON).Poke(40, "sample"))

			frame := make([]byte, 65536)
			readCount, err := variance.Read(frame)

			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](variance.artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestVarianceWrite(t *testing.T) {
	Convey("Given a Variance", t, func() {
		variance := NewVariance(datura.Acquire("variance-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(variance, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
			So(err, ShouldBeIn, nil, io.EOF)
		})
	})
}
