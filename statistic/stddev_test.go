package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestStdDevRead(t *testing.T) {
	Convey("Given a StdDev", t, func() {
		stdDev := NewStdDev(scalarStageConfig("stddev-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		err := nomagique.RoundTripArtifact(ScalarWire(artifact, "sample", 1), stdDev)

		Convey("When the first sample arrives", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestStdDevSeries(t *testing.T) {
	Convey("Given a StdDev stage", t, func() {
		stdDev := NewStdDev(scalarStageConfig("stddev-config-series"))
		artifact := datura.Acquire("test", datura.APPJSON)
		var got float64

		for _, sample := range []float64{1, 2, 3, 4} {
			err := nomagique.RoundTripArtifact(ScalarWire(artifact, "sample", sample), stdDev)

			if err != nil {
				continue
			}

			got = datura.Peek[float64](artifact, "output", "value")
		}

		Convey("It should derive dispersion from history", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkStdDevRead(testingTB *testing.B) {
	stdDev := NewStdDev(scalarStageConfig("stddev-bench"))
	artifact := datura.Acquire("stddev-bench", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = nomagique.RoundTripArtifact(ScalarWire(artifact, "sample", 2.0), stdDev)
	}
}
