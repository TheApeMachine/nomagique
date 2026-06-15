package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRank(testingTB *testing.T) {
	Convey("Given Rank constructor", testingTB, func() {
		empirical := NewRank()

		Convey("It should return a usable dynamic", func() {
			So(empirical, ShouldNotBeNil)
		})
	})
}

func TestEmpiricalRank_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		empirical := NewRank()

		Convey("It should return zero output", func() {
			So(observeInputs(empirical), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		empirical := NewRank()
		before := observeInputs(empirical, 10)

		Convey("It should leave output unchanged", func() {
			So(observeWithoutSample(empirical, 99), ShouldEqual, before)
		})
	})

	Convey("Given empirical rank history", testingTB, func() {
		empirical := NewRank()
		_ = observeInputs(empirical, 10)
		got := observeInputs(empirical, 5)

		Convey("It should return a probability in the unit interval", func() {
			So(got, ShouldBeGreaterThan, 0)
			So(got, ShouldBeLessThan, 1)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		empirical := NewRank()
		_ = observeInputs(empirical, 10)

		Convey("It should match a single combined scalar", func() {
			withWork := observeWithWork(empirical, 5, 3)
			combined := NewRank()
			_ = observeInputs(combined, 10)
			direct := observeInputs(combined, 8)

			So(withWork, ShouldEqual, direct)
		})
	})
}

func TestEmpiricalRank_Reset(testingTB *testing.T) {
	Convey("Given an observed rank", testingTB, func() {
		empirical := NewRank()
		_ = observeInputs(empirical, 10)

		So(empirical.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(empirical.state.Ready, ShouldBeFalse)
			So(observeInputs(empirical), ShouldEqual, 0)
		})
	})
}

func BenchmarkRank_Observe(testingTB *testing.B) {
	empirical := NewRank()
	_ = observeInputs(empirical, 10)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(empirical, 10.5)
	}
}
