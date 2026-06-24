package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func bivariateMomentConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "sampleKey").
		Poke("paired", "pairedKey").
		Poke("value", "outputKey").
		Poke(1.0, "config", "r").
		Poke(1.0, "config", "s")
}

func TestBivariateMomentRead(t *testing.T) {
	Convey("Given a BivariateMoment stage", t, func() {
		bivariateMoment := NewBivariateMoment(bivariateMomentConfig("bm-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		xValues := []float64{1, 2, 3, 4}
		yValues := []float64{2, 5, 7, 10}

		for index := 0; index < len(xValues); index++ {
			wired := PairWire(artifact, "sample", "paired", xValues[index], yValues[index])
			err := transport.NewFlipFlop(wired, bivariateMoment)

			if index < 1 {
				So(err, ShouldNotBeNil)
				continue
			}

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should compute the mixed moment", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkBivariateMomentRead(testingTB *testing.B) {
	bivariateMoment := NewBivariateMoment(bivariateMomentConfig("bm-config-bench"))
	artifact := datura.Acquire("bm-bench", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		wired := PairWire(artifact, "sample", "paired", 2.0, 5.0)
		_ = transport.NewFlipFlop(wired, bivariateMoment)
	}
}
