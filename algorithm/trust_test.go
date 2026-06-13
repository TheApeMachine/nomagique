package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestTrust_Observe(testingTB *testing.T) {
	Convey("Given accurate predicted and actual pairs", testingTB, func() {
		trust := NewTrust()

		for step := 0; step < 16; step++ {
			predicted := float64(step + 10)
			_ = trust.Observe(core.Float64(predicted), core.Float64(predicted))
		}

		score := trust.Observe(core.Float64(26), core.Float64(26))

		Convey("It should return positive trust-weighted calibration", func() {
			So(float64(score), ShouldBeGreaterThan, 0)
			So(trust.Scale(), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkTrust_Observe(testingTB *testing.B) {
	trust := NewTrust()

	for testingTB.Loop() {
		step := float64(testingTB.N % 16)
		_ = trust.Observe(core.Float64(step+1), core.Float64((step+1)*2))
	}
}
