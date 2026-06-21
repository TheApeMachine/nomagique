package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/tests"
)

func encodeGapBatch(
	syncLeft, syncRight, asyncLeft, asyncRight []float64,
) []float64 {
	if len(syncLeft) != len(syncRight) {
		return nil
	}

	if len(asyncLeft) == 0 || len(asyncLeft)%2 != 0 || len(asyncLeft) != len(asyncRight) {
		return nil
	}

	batch := make(
		[]float64,
		0,
		2+len(syncLeft)+len(syncRight)+len(asyncLeft)+len(asyncRight),
	)
	batch = append(batch, float64(len(syncLeft)), float64(len(asyncLeft)/2))
	batch = append(batch, syncLeft...)
	batch = append(batch, syncRight...)
	batch = append(batch, asyncLeft...)
	batch = append(batch, asyncRight...)

	return batch
}

func TestCorrelate_Observe(testingTB *testing.T) {
	Convey("Given positively coupled sync and async streams", testingTB, func() {
		batch := encodeGapBatch(
			[]float64{1, 2, 3, 4, 5, 6},
			[]float64{2, 4, 6, 8, 10, 12},
			[]float64{0, 100, 1, 110, 2, 121, 3, 133.1},
			[]float64{0, 50, 1, 55, 2, 60.5, 3, 66.55},
		)
		correlate := NewCorrelate(
			datura.Acquire("test-config", datura.APPJSON).
				Poke(1.0, "config", "maxIntervalSeconds"),
		)
		So(tests.WriteSamples(correlate, batch...), ShouldBeNil)

		outbound := readOutbound(correlate)

		pearson := datura.Peek[float64](outbound, "output", "pearson")
		hayashi := datura.Peek[float64](outbound, "output", "hayashi")
		gap := datura.Peek[float64](outbound, "output", "gap")

		Convey("It should report positive synchronous correlation", func() {
			So(pearson, ShouldBeGreaterThan, 0.9)
		})

		Convey("It should expose a finite async-sync gap", func() {
			So(hayashi, ShouldAlmostEqual, 1, 1e-6)
			So(gap, ShouldEqual, hayashi-pearson)
		})
	})
}

func BenchmarkCorrelate_Observe(testingTB *testing.B) {
	batch := encodeGapBatch(
		[]float64{1, 2, 3, 4, 5, 6, 7, 8},
		[]float64{2, 4, 6, 8, 10, 12, 14, 16},
		[]float64{0, 100, 1, 110, 2, 120, 3, 130, 4, 140, 5, 150, 6, 160, 7, 170},
		[]float64{0, 100, 1, 120, 2, 140, 3, 160, 4, 180, 5, 200, 6, 220, 7, 240},
	)
	correlate := NewCorrelate(
		datura.Acquire("test-config", datura.APPJSON).Poke(1.0, "config", "maxIntervalSeconds"),
	)
	frame := make([]byte, 4096)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = tests.WriteSamples(correlate, batch...)
		_, _ = correlate.Read(frame)
	}
}
