package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestPanel_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		key    float64
		value  float64
		expect float64
	}{
		{"register member", 1, 0.02, 0.02},
		{"update member", 2, 0.04, 0.04},
		{"negative sample", 3, -0.01, -0.01},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			panel := Panel[float64]{}
			got := panel.Observe(
				core.Scalar[float64](testCase.key),
				core.Scalar[float64](testCase.value),
			)

			Convey("It should store and echo the sample", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given fewer than two scalar inputs", testingTB, func() {
		panel := Panel[float64]{}
		_ = panel.Observe(core.Scalar[float64](1), core.Scalar[float64](0.02))

		Convey("It should leave output unchanged", func() {
			So(float64(panel.Observe(core.Scalar[float64](1))), ShouldEqual, 0.02)
		})
	})

	Convey("Given a non-scalar input", testingTB, func() {
		panel := Panel[float64]{}
		_ = panel.Observe(core.Scalar[float64](1), core.Scalar[float64](0.02))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should not overwrite stored values", func() {
			So(float64(panel.Observe(stage)), ShouldEqual, 0.02)
		})
	})
}

func TestPanel_Reset(testingTB *testing.T) {
	Convey("Given a populated panel", testingTB, func() {
		panel := Panel[float64]{}
		_ = panel.Observe(core.Scalar[float64](1), core.Scalar[float64](0.02))

		So(panel.Reset(), ShouldBeNil)

		Convey("It should clear stored members", func() {
			leaveOneOut := NewLeaveOneOutMedian(&panel)

			So(float64(leaveOneOut.Observe(core.Scalar[float64](1))), ShouldEqual, 0)
		})
	})
}

func TestLeaveOneOutMedian_Observe(testingTB *testing.T) {
	Convey("Given a populated panel", testingTB, func() {
		panel := Panel[float64]{}
		leaveOneOut := NewLeaveOneOutMedian(&panel)

		_ = panel.Observe(core.Scalar[float64](1), core.Scalar[float64](0.02))
		_ = panel.Observe(core.Scalar[float64](2), core.Scalar[float64](0.04))
		_ = panel.Observe(core.Scalar[float64](3), core.Scalar[float64](0.06))

		got := leaveOneOut.Observe(core.Scalar[float64](1))

		Convey("It should median peer values", func() {
			So(float64(got), ShouldEqual, 0.05)
		})
	})

	Convey("Given a composed macro number", testingTB, func() {
		panel := Panel[float64]{}
		leaveOneOut := NewLeaveOneOutMedian(&panel)

		_ = panel.Observe(core.Scalar[float64](1), core.Scalar[float64](0.02))
		_ = panel.Observe(core.Scalar[float64](2), core.Scalar[float64](0.04))
		_ = panel.Observe(core.Scalar[float64](3), core.Scalar[float64](0.06))

		macro := float64(core.Scalar[float64](1).Observe(leaveOneOut))

		Convey("It should match direct observation", func() {
			So(macro, ShouldEqual, 0.05)
		})
	})

	Convey("Given an empty panel", testingTB, func() {
		panel := Panel[float64]{}
		leaveOneOut := NewLeaveOneOutMedian(&panel)

		Convey("It should return zero", func() {
			So(float64(leaveOneOut.Observe(core.Scalar[float64](1))), ShouldEqual, 0)
		})
	})
}

func TestLeaveOneOutMedian_Reset(testingTB *testing.T) {
	Convey("Given an observed leave-one-out stage", testingTB, func() {
		panel := Panel[float64]{}
		leaveOneOut := NewLeaveOneOutMedian(&panel)

		_ = panel.Observe(core.Scalar[float64](1), core.Scalar[float64](0.02))
		_ = panel.Observe(core.Scalar[float64](2), core.Scalar[float64](0.04))
		_ = leaveOneOut.Observe(core.Scalar[float64](1))

		So(leaveOneOut.Reset(), ShouldBeNil)

		Convey("It should clear derived output but keep panel data", func() {
			So(float64(leaveOneOut.Observe()), ShouldEqual, 0)
			So(float64(leaveOneOut.Observe(core.Scalar[float64](2))), ShouldEqual, 0.02)
		})
	})
}

func BenchmarkLeaveOneOutMedian_Observe(b *testing.B) {
	panel := Panel[float64]{}
	leaveOneOut := NewLeaveOneOutMedian(&panel)

	_ = panel.Observe(core.Scalar[float64](1), core.Scalar[float64](0.02))
	_ = panel.Observe(core.Scalar[float64](2), core.Scalar[float64](0.04))
	_ = panel.Observe(core.Scalar[float64](3), core.Scalar[float64](0.06))

	b.ReportAllocs()

	for b.Loop() {
		_ = leaveOneOut.Observe(core.Scalar[float64](1))
	}
}
