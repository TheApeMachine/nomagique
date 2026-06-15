package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMultiverse_Observe(testingTB *testing.T) {
	Convey("Given correlated window sets", testingTB, func() {
		first := NewWindowSet(16)
		second := NewWindowSet(16)

		for step := range 16 {
			observeEpochLevel(first, int64((step+1)*1_000), 100+float64(step)*0.1)
			observeEpochLevel(second, int64((step+1)*1_000), 50+float64(step)*0.05)
		}

		multiverse := NewMultiverse(
			[]*WindowSet{first, second},
			TierWindows{Fast: 4, Medium: 8, Slow: 16},
			ContagionConfig{MinSamples: 2, MemberCap: 2, AdaptiveSigma: 2},
		)

		coupling := observeInputs(multiverse)
		readings := multiverse.TierReadings()

		Convey("It should publish positive coupling", func() {
			So(float64(coupling), ShouldBeGreaterThan, 0)
			So(readings.Fast, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkMultiverse_Observe(testingTB *testing.B) {
	sets := make([]*WindowSet, 8)

	for index := range sets {
		set := NewWindowSet(32)

		for step := range 32 {
			observeEpochLevel(set, int64((step+1)*1_000), 100+float64(index)+float64(step)*0.01)
		}

		sets[index] = set
	}

	multiverse := NewMultiverse(
		sets,
		TierWindows{Fast: 8, Medium: 16, Slow: 32},
		ContagionConfig{MinSamples: 4, MemberCap: 8},
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(multiverse)
	}
}
