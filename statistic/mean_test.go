package statistic

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

var meanConfig = datura.Acquire("mean-config", datura.APPJSON)

var meanInput = datura.Acquire("test", datura.APPJSON).Poke(1, "sample")

func TestMeanRead(t *testing.T) {
	Convey("Given a Mean", t, func() {
		mean := NewMean(meanConfig)
		_, err := io.Copy(mean, meanInput)

		So(err, ShouldBeNil)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := mean.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](mean.artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}

func TestMeanWrite(t *testing.T) {
	Convey("Given a Mean", t, func() {
		mean := NewMean(datura.Acquire("mean-config-write", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(mean, meanInput)
			So(err, ShouldBeNil)
		})
	})
}

func TestMeanSeries(t *testing.T) {
	Convey("Given a sample series", t, func() {
		mean := NewMean(datura.Acquire("mean-config-series", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{1, 2, 3, 4} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, mean)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return the running mean", func() {
			So(got, ShouldEqual, 2.5)
		})
	})
}

func BenchmarkMeanRead(b *testing.B) {
	mean := NewMean(datura.Acquire("mean-config-bench", datura.APPJSON))
	_, _ = io.Copy(mean, meanInput)
	frame := make([]byte, 65536)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = mean.Read(frame)
	}
}
