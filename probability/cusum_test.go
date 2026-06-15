package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestCUSUM(testingTB *testing.T) {
	Convey("Given CUSUM constructor", testingTB, func() {
		changeSum := CUSUM[float64]()

		Convey("It should return a usable dynamic", func() {
			So(changeSum, ShouldNotBeNil)
		})
	})
}

func TestChangeSum_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		changeSum := CUSUM[float64]()

		Convey("It should return zero output", func() {
			So(changeSum.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		changeSum := CUSUM[float64]()
		before := changeSum.Observe(core.Scalar[float64](10))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(changeSum.Observe(stage), ShouldEqual, before)
		})
	})

	Convey("Given a change sum", testingTB, func() {
		changeSum := CUSUM[float64]()
		_ = changeSum.Observe(core.Scalar[float64](10))
		got := changeSum.Observe(core.Scalar[float64](25))

		Convey("It should accumulate evidence", func() {
			So(float64(got), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		changeSum := CUSUM[float64]()
		_ = changeSum.Observe(core.Scalar[float64](10))

		Convey("It should match a single combined scalar", func() {
			withWork := changeSum.Observe(
				core.Scalar[float64](5),
				core.Scalar[float64](3),
			)
			combined := CUSUM[float64]()
			_ = combined.Observe(core.Scalar[float64](10))
			direct := combined.Observe(core.Scalar[float64](8))

			So(withWork, ShouldEqual, direct)
		})
	})
}

func TestChangeSum_Reset(testingTB *testing.T) {
	Convey("Given an observed change sum", testingTB, func() {
		changeSum := CUSUM[float64]()
		_ = changeSum.Observe(core.Scalar[float64](10))

		So(changeSum.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(changeSum.state.Ready, ShouldBeFalse)
			So(float64(changeSum.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkCUSUM_Observe(testingTB *testing.B) {
	changeSum := CUSUM[float64]()
	_ = changeSum.Observe(core.Scalar[float64](10))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = changeSum.Observe(core.Scalar[float64](10.5))
	}
}
