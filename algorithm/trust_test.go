package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTrust_Observe(testingTB *testing.T) {
	Convey("Given accurate predicted and actual pairs", testingTB, func() {
		trust := NewTrust()

		for step := 0; step < 16; step++ {
			predicted := float64(step + 10)
			_ = observeWithWork(trust, predicted, predicted)
		}

		score := observeWithWork(trust, 26, 26)

		Convey("It should return positive trust-weighted calibration", func() {
			So(score, ShouldBeGreaterThan, 0)
			So(trust.Scale(), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkTrust_Observe(testingTB *testing.B) {
	trust := NewTrust()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		step := float64(testingTB.N % 16)
		_ = observeWithWork(trust, step+1, (step+1)*2)
	}
}
