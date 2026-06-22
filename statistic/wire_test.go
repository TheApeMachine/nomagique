package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestWireScalar(t *testing.T) {
	Convey("Given wire navigation config", t, func() {
		config := datura.Acquire("wire-config", datura.APPJSON).
			Poke("features", "root").
			Poke([]string{"bid", "ask"}, "inputs")

		Convey("It should read features by named inputs", func() {
			state := datura.Acquire("wire-state", datura.APPJSON).
				Poke([]float64{1.1, 2.2}, "features").
				Poke([]string{"bid", "ask"}, "inputs").
				Poke("features", "root")

			bid, err := WireScalar(config, state, "bid")

			So(err, ShouldBeNil)
			So(bid, ShouldEqual, 1.1)
		})

		Convey("It should read output root keys", func() {
			state := datura.Acquire("wire-output-state", datura.APPJSON).
				Poke(0.42, "output", "value").
				Poke("output", "root")

			value, err := WireScalarAt(config, state, "output", "value")

			So(err, ShouldBeNil)
			So(value, ShouldEqual, 0.42)
		})

		Convey("It should reject missing keys", func() {
			state := datura.Acquire("wire-missing-state", datura.APPJSON)

			_, err := WireScalar(config, state, "bid")

			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkWireScalar(b *testing.B) {
	config := datura.Acquire("wire-bench-config", datura.APPJSON).
		Poke("sample", "input")
	state := datura.Acquire("wire-bench-state", datura.APPJSON).
		Poke(1.23, "sample")

	for b.Loop() {
		_, _ = WireScalar(config, state, "sample")
	}
}
