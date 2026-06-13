package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestCovariance_Observe(testingTB *testing.T) {
	Convey("Given positively coupled streams", testingTB, func() {
		left := nomagique.Numbers(1, 2, 3, 4)
		right := nomagique.Numbers(2, 4, 6, 8)
		inputs := append(left, right...)
		value := NewCovariance(nil).Observe(inputs...)

		Convey("It should return positive covariance", func() {
			So(float64(value), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkCovariance_Observe(testingTB *testing.B) {
	left := nomagique.Numbers(1, 2, 3, 4, 5, 6, 7, 8)
	right := nomagique.Numbers(2, 4, 6, 8, 10, 12, 14, 16)
	inputs := append(left, right...)
	covariance := NewCovariance(nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = covariance.Observe(inputs...)
	}
}
