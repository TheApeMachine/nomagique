package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func klConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "sampleKey").
		Poke("paired", "pairedKey").
		Poke("value", "outputKey")
}

func TestKLDivergenceRead(t *testing.T) {
	Convey("Given a KL stage", t, func() {
		kl := NewKLDivergence(klConfig("kl-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		observed := []float64{1, 1, 1, 1}
		expected := []float64{1, 1, 1, 1}

		for index := 0; index < len(observed); index++ {
			wired := PairWire(artifact, "sample", "paired", observed[index], expected[index])
			err := transport.NewFlipFlop(wired, kl)

			if index < 1 {
				So(err, ShouldNotBeNil)
				continue
			}

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return zero divergence for identical mass", func() {
			So(got, ShouldAlmostEqual, 0, 1e-9)
		})
	})

	Convey("Given mismatched distributions after renormalization", t, func() {
		kl := NewKLDivergence(klConfig("kl-config-mismatch"))
		artifact := datura.Acquire("test", datura.APPJSON)
		observed := []float64{4, 1}
		expected := []float64{1, 4}

		for index := range observed {
			wired := PairWire(artifact, "sample", "paired", observed[index], expected[index])
			err := transport.NewFlipFlop(wired, kl)

			if index < 1 {
				So(err, ShouldNotBeNil)
				continue
			}

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return a positive divergence", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})
}
