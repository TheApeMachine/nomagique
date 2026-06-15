package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCUSUM(testingTB *testing.T) {
	Convey("Given CUSUM constructor", testingTB, func() {
		changeSum := NewCUSUM()

		Convey("It should return a usable dynamic", func() {
			So(changeSum, ShouldNotBeNil)
		})
	})
}

func TestChangeSum_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		changeSum := NewCUSUM()

		Convey("It should return zero output", func() {
			So(observeInputs(changeSum), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		changeSum := NewCUSUM()
		before := observeInputs(changeSum, 10)

		Convey("It should leave output unchanged", func() {
			So(observeWithoutSample(changeSum, 99), ShouldEqual, before)
		})
	})

	Convey("Given a change sum", testingTB, func() {
		changeSum := NewCUSUM()
		_ = observeInputs(changeSum, 10)
		got := observeInputs(changeSum, 25)

		Convey("It should accumulate evidence", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		changeSum := NewCUSUM()
		_ = observeInputs(changeSum, 10)

		Convey("It should match a single combined scalar", func() {
			withWork := observeWithWork(changeSum, 5, 3)
			combined := NewCUSUM()
			_ = observeInputs(combined, 10)
			direct := observeInputs(combined, 8)

			So(withWork, ShouldEqual, direct)
		})
	})
}

func TestChangeSum_Reset(testingTB *testing.T) {
	Convey("Given an observed change sum", testingTB, func() {
		changeSum := NewCUSUM()
		_ = observeInputs(changeSum, 10)

		So(changeSum.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(changeSum.state.Ready, ShouldBeFalse)
			So(observeInputs(changeSum), ShouldEqual, 0)
		})
	})
}

func BenchmarkCUSUM_Observe(testingTB *testing.B) {
	changeSum := NewCUSUM()
	_ = observeInputs(changeSum, 10)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(changeSum, 10.5)
	}
}
