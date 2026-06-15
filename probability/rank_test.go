package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestRank(testingTB *testing.T) {
	Convey("Given Rank constructor", testingTB, func() {
		empirical := Rank[float64]()

		Convey("It should return a usable dynamic", func() {
			So(empirical, ShouldNotBeNil)
		})
	})
}

func TestEmpiricalRank_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		empirical := Rank[float64]()

		Convey("It should return zero output", func() {
			So(empirical.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		empirical := Rank[float64]()
		before := empirical.Observe(core.Scalar[float64](10))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(empirical.Observe(stage), ShouldEqual, before)
		})
	})

	Convey("Given empirical rank history", testingTB, func() {
		empirical := Rank[float64]()
		_ = empirical.Observe(core.Scalar[float64](10))
		got := empirical.Observe(core.Scalar[float64](5))

		Convey("It should return a probability in the unit interval", func() {
			So(float64(got), ShouldBeGreaterThan, 0)
			So(float64(got), ShouldBeLessThan, 1)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		empirical := Rank[float64]()
		_ = empirical.Observe(core.Scalar[float64](10))

		Convey("It should match a single combined scalar", func() {
			withWork := empirical.Observe(
				core.Scalar[float64](5),
				core.Scalar[float64](3),
			)
			combined := Rank[float64]()
			_ = combined.Observe(core.Scalar[float64](10))
			direct := combined.Observe(core.Scalar[float64](8))

			So(withWork, ShouldEqual, direct)
		})
	})
}

func TestEmpiricalRank_Reset(testingTB *testing.T) {
	Convey("Given an observed rank", testingTB, func() {
		empirical := Rank[float64]()
		_ = empirical.Observe(core.Scalar[float64](10))

		So(empirical.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(empirical.state.Ready, ShouldBeFalse)
			So(float64(empirical.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkRank_Observe(testingTB *testing.B) {
	empirical := Rank[float64]()
	_ = empirical.Observe(core.Scalar[float64](10))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = empirical.Observe(core.Scalar[float64](10.5))
	}
}
