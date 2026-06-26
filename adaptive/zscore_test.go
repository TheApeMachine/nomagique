package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestZScoreRead(t *testing.T) {
	Convey("Given a ZScore", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When the first sample arrives", func() {
			_, err := nomagique.WriteArtifact(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))

			So(err, ShouldBeIn, nil, io.EOF)

			_, err = surprise.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})

		Convey("When distinct samples arrive", func() {
			_, _ = nomagique.WriteArtifact(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			_, _ = surprise.Read(make([]byte, 65536))
			_, _ = nomagique.WriteArtifact(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 22))
			_, _ = surprise.Read(make([]byte, 65536))
			_, _ = nomagique.WriteArtifact(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 30))
			_, _ = surprise.Read(make([]byte, 65536))
			artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 40)
			_, _ = nomagique.WriteArtifact(surprise, artifact)

			frame := make([]byte, 65536)
			readCount, err := surprise.Read(frame)

			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)

			err = nomagique.RoundTripArtifact(artifact, surprise)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldNotEqual, 0)
		})
	})

	Convey("Given a non-finite sample", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON).Poke("sample", "input"))
		invalid := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", math.NaN())
		_, err := nomagique.WriteArtifact(surprise, invalid)

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
		_, _ = nomagique.WriteArtifact(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
		_, _ = surprise.Read(make([]byte, 65536))
		_, _ = nomagique.WriteArtifact(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 22))
		_, _ = surprise.Read(make([]byte, 65536))
		_, _ = nomagique.WriteArtifact(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 30))
		_, _ = surprise.Read(make([]byte, 65536))
		artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 40)
		_, _ = nomagique.WriteArtifact(surprise, artifact)

		frame := make([]byte, 65536)
		readCount, err := surprise.Read(frame)

		Convey("It should anchor at zero without treating it as missing", func() {
			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)

			err = nomagique.RoundTripArtifact(artifact, surprise)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldNotEqual, 0)
		})
	})
}

func TestZScoreWrite(t *testing.T) {
	Convey("Given a ZScore", t, func() {
		surprise := NewZScore(datura.Acquire("zscore-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When Write is called", func() {
			_, err := nomagique.WriteArtifact(surprise, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			So(err, ShouldBeIn, nil, io.EOF)
		})
	})
}

func BenchmarkZScore_Read(benchmark *testing.B) {
	surprise := &ZScore{
		artifact: datura.Acquire("zscore-config", datura.APPJSON),
		mean:     20,
		variance: 1,
		prev:     30,
		min:      10,
		max:      30,
		count:    2,
	}

	benchmark.ResetTimer()

	for benchmark.Loop() {
		artifact := ScalarWire(datura.Acquire("zscore-bench", datura.APPJSON), "sample", 40)

		if err := nomagique.RoundTripArtifact(artifact, surprise); err != nil && err != io.EOF {
			benchmark.Fatal(err)
		}
	}
}
