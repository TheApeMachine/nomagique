package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestTrust_Observe(testingTB *testing.T) {
	Convey("Given accurate predicted and actual pairs", testingTB, func() {
		trust := NewTrust[float64]()

		for step := 0; step < 16; step++ {
			predicted := float64(step + 10)
			_ = trust.Observe(
				core.Scalar[float64](predicted),
				core.Scalar[float64](predicted),
			)
		}

		score := trust.Observe(
			core.Scalar[float64](26),
			core.Scalar[float64](26),
		)

		Convey("It should return positive trust-weighted calibration", func() {
			So(float64(score), ShouldBeGreaterThan, 0)
			So(trust.Scale(), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkTrust_Observe(testingTB *testing.B) {
	trust := NewTrust[float64]()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		step := float64(testingTB.N % 16)
		_ = trust.Observe(
			core.Scalar[float64](step+1),
			core.Scalar[float64]((step+1)*2),
		)
	}
}
