package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBackdoor_Observe(testingTB *testing.T) {
	Convey("Given aligned node streams with causal structure", testingTB, func() {
		backdoor := NewBackdoor(3, 2, []int{0, 1}, 12)
		effect := observeInputs(backdoor,
			0.1, 0.2, 0.5, 0.05,
			0.2, 0.4, 1.0, 0.1,
			0.3, 0.6, 1.5, 0.15,
		)

		Convey("It should return a finite backdoor effect", func() {
			So(float64(effect), ShouldNotEqual, 0)
		})
	})
}

func BenchmarkBackdoor_Observe(testingTB *testing.B) {
	backdoor := NewBackdoor(3, 2, []int{0, 1}, 12)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(backdoor,
			0.1, 0.2, 0.5, 0.05,
			0.2, 0.4, 1.0, 0.1,
			0.3, 0.6, 1.5, 0.15,
		)
	}
}
