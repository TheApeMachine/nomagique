package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMedianPairwiseAbsCorrelation(testingTB *testing.T) {
	Convey("Given proportional interval series", testingTB, func() {
		left := NewIntervalSeries(8)
		right := NewIntervalSeries(8)

		left.Observe(1_000, 100)
		left.Observe(2_000, 110)
		right.Observe(1_000, 50)
		right.Observe(2_000, 55)

		Convey("It should return unit median correlation", func() {
			value := MedianPairwiseAbsCorrelation([]*IntervalSeries{left, right})
			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func TestContagionObserve(testingTB *testing.T) {
	Convey("Given a contagion estimator", testingTB, func() {
		contagion := NewContagion(ContagionConfig{
			MinSamples:    1,
			SymbolCap:     2,
			AdaptiveSigma: 2,
		})

		snapshot := func(base float64) WindowSnapshot {
			first := NewIntervalSeries(8)
			second := NewIntervalSeries(8)

			first.Observe(1_000, base)
			first.Observe(2_000, base*1.1)
			second.Observe(1_000, base/2)
			second.Observe(2_000, base*0.55)

			return WindowSnapshot{
				Fast:   first,
				Medium: first.Clone(),
				Slow:   first.Clone(),
			}
		}

		Convey("It should publish positive coupling for correlated tiers", func() {
			value := contagion.Observe([]WindowSnapshot{
				snapshot(100),
				snapshot(200),
			})

			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkContagionObserve(testingTB *testing.B) {
	contagion := NewContagion(ContagionConfig{
		MinSamples: 8,
		SymbolCap:  16,
	})

	snapshots := make([]WindowSnapshot, 16)

	for index := range snapshots {
		series := NewIntervalSeries(32)

		for step := range 32 {
			series.Observe(int64((step+1)*1_000), 100+float64(index)+float64(step)*0.01)
		}

		snapshots[index] = WindowSnapshot{
			Fast:   series.CloneTail(8),
			Medium: series.CloneTail(16),
			Slow:   series.Clone(),
		}
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = contagion.Observe(snapshots)
	}
}
