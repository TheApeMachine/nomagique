package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestZScoreRead(t *testing.T) {
	Convey("Given a ZScore", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When the first sample arrives", func() {
			_, err := io.Copy(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))

			So(err, ShouldBeIn, nil, io.EOF)

			frame := make([]byte, 65536)
			readCount, err := surprise.Read(frame)

			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)

			outbound := datura.Acquire("zscore-outbound", datura.APPJSON)
			_, _ = outbound.Write(frame[:readCount])
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
		})

		Convey("When warmed up with distinct samples", func() {
			_, _ = io.Copy(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			_, _ = surprise.Read(make([]byte, 65536))
			_, _ = io.Copy(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 22))
			_, _ = surprise.Read(make([]byte, 65536))
			_, _ = io.Copy(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 30))
			_, _ = surprise.Read(make([]byte, 65536))
			artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 40)
			_, _ = io.Copy(surprise, artifact)

			frame := make([]byte, 65536)
			readCount, err := surprise.Read(frame)

			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)

			err = transport.NewFlipFlop(artifact, surprise)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldNotEqual, 0)
		})
	})

	Convey("Given a non-finite sample", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON).Poke("sample", "input"))
		invalid := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", math.NaN())
		_, err := io.Copy(surprise, invalid)

		So(err, ShouldBeNil)

		Convey("When Read is called", func() {
			_, err := surprise.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given an explicit zero anchor", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON).Poke("sample", "input").
			Poke("explicit", "anchorMode").
			Poke(0.0, "anchor"))
		_, _ = io.Copy(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
		_, _ = surprise.Read(make([]byte, 65536))
		_, _ = io.Copy(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 22))
		_, _ = surprise.Read(make([]byte, 65536))
		_, _ = io.Copy(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 30))
		_, _ = surprise.Read(make([]byte, 65536))
		artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 40)
		_, _ = io.Copy(surprise, artifact)

		frame := make([]byte, 65536)
		readCount, err := surprise.Read(frame)

		Convey("It should anchor at zero without treating it as missing", func() {
			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)

			err = transport.NewFlipFlop(artifact, surprise)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldNotEqual, 0)
		})
	})
}

func TestZScoreWrite(t *testing.T) {
	Convey("Given a ZScore", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When Write is called", func() {
			_, err := io.Copy(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			So(err, ShouldBeIn, nil, io.EOF)
		})
	})
}

func BenchmarkZScore_Read(benchmark *testing.B) {
	surprise := &ZScore{
		artifact:     datura.Acquire("zscore-config", datura.APPJSON),
		bootstrapped: true,
		mean:         20,
		variance:     1,
		prev:         30,
		min:          10,
		max:          30,
	}

	benchmark.ResetTimer()

	for benchmark.Loop() {
		artifact := ScalarWire(datura.Acquire("zscore-bench", datura.APPJSON), "sample", 40)

		if err := transport.NewFlipFlop(artifact, surprise); err != nil && err != io.EOF {
			benchmark.Fatal(err)
		}
	}
}
