package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMedianPairwiseAbsCorrelation(testingTB *testing.T) {
	Convey("Given proportional interval series", testingTB, func() {
		left := NewIntervalSeries[float64](8)
		right := NewIntervalSeries[float64](8)

		observeEpochLevel(left, 1_000, 100)
		observeEpochLevel(left, 2_000, 110)
		observeEpochLevel(right, 1_000, 50)
		observeEpochLevel(right, 2_000, 55)

		Convey("It should return unit median correlation", func() {
			value := MedianPairwiseAbsCorrelation([]*IntervalSeries[float64]{left, right})
			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func TestContagionObserve(testingTB *testing.T) {
	Convey("Given a contagion stage with fed window sets", testingTB, func() {
		first := NewWindowSet[float64](8)
		second := NewWindowSet[float64](8)

		observeEpochLevel(first, 1_000, 100)
		observeEpochLevel(first, 2_000, 110)
		observeEpochLevel(second, 1_000, 50)
		observeEpochLevel(second, 2_000, 55)

		contagion := NewContagion(
			[]*WindowSet[float64]{first, second},
			TierWindows{Fast: 8, Medium: 8, Slow: 8},
			ContagionConfig{
				MinSamples:    1,
				MemberCap:     2,
				AdaptiveSigma: 2,
			},
		)

		Convey("It should publish positive coupling for correlated tiers", func() {
			value := float64(contagion.Observe())
			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkContagionObserve(testingTB *testing.B) {
	sets := make([]*WindowSet[float64], 16)

	for index := range sets {
		set := NewWindowSet[float64](32)

		for step := range 32 {
			observeEpochLevel(set, int64((step+1)*1_000), 100+float64(index)+float64(step)*0.01)
		}

		sets[index] = set
	}

	contagion := NewContagion(
		sets,
		TierWindows{Fast: 8, Medium: 16, Slow: 32},
		ContagionConfig{
			MinSamples: 8,
			MemberCap:  16,
		},
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = contagion.Observe()
	}
}
