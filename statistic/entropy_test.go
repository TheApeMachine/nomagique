package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestEntropy_Observe(testingTB *testing.T) {
	Convey("Given a uniform mass distribution", testingTB, func() {
		value := NewEntropy(0).Observe(nomagique.Numbers(1, 1, 1, 1)...)

		Convey("It should return log(4)", func() {
			So(float64(value), ShouldAlmostEqual, math.Log(4), 1e-9)
		})
	})

	Convey("Given a peaked mass distribution", testingTB, func() {
		uniform := NewEntropy(0).Observe(nomagique.Numbers(1, 1, 1, 1)...)
		peaked := NewEntropy(0).Observe(nomagique.Numbers(100, 1, 1, 1)...)

		Convey("It should return lower entropy than a uniform distribution", func() {
			So(float64(peaked), ShouldBeLessThan, float64(uniform))
		})
	})
}

func BenchmarkEntropy_Observe(testingTB *testing.B) {
	inputs := nomagique.Numbers(1, 2, 3, 4, 5, 6, 7, 8)
	entropy := NewEntropy(0)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = entropy.Observe(inputs...)
	}
}
