package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestZScoreRead(t *testing.T) {
	Convey("Given a ZScore", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON))

		Convey("When the first sample arrives", func() {
			_, err := io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))

			So(err, ShouldBeIn, nil, io.EOF)

			_, err = surprise.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})

		Convey("When warmed up with distinct samples", func() {
			_, _ = io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
			_, _ = surprise.Read(make([]byte, 65536))
			_, _ = io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(22, "sample"))
			_, _ = surprise.Read(make([]byte, 65536))
			_, _ = io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(30, "sample"))
			_, _ = surprise.Read(make([]byte, 65536))
			_, _ = io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(40, "sample"))

			frame := make([]byte, 65536)
			readCount, err := surprise.Read(frame)

			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](surprise.artifact, "output", "value"), ShouldNotEqual, 0)
		})
	})

	Convey("Given a non-finite sample", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON))
		invalid := datura.Acquire("test", datura.APPJSON).Poke(math.NaN(), "sample")
		_, err := io.Copy(surprise, invalid)

		So(err, ShouldBeNil)

		Convey("When Read is called", func() {
			_, err := surprise.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given an explicit zero anchor", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON).
			Poke("explicit", "anchorMode").
			Poke(0.0, "anchor"))
		_, _ = io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
		_, _ = surprise.Read(make([]byte, 65536))
		_, _ = io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(22, "sample"))
		_, _ = surprise.Read(make([]byte, 65536))
		_, _ = io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(30, "sample"))
		_, _ = surprise.Read(make([]byte, 65536))
		_, _ = io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(40, "sample"))

		frame := make([]byte, 65536)
		readCount, err := surprise.Read(frame)

		Convey("It should anchor at zero without treating it as missing", func() {
			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](surprise.artifact, "output", "value"), ShouldNotEqual, 0)
		})
	})
}

func TestZScoreWrite(t *testing.T) {
	Convey("Given a ZScore", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(surprise, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
			So(err, ShouldBeIn, nil, io.EOF)
		})
	})
}
