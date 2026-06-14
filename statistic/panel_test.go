package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestPanelObserve(t *testing.T) {
	Convey("Given a panel", t, func() {
		panel := Panel{}

		Convey("When a member sample is observed", func() {
			result := panel.Observe(nomagique.Scalar(1), nomagique.Scalar(0.02))

			Convey("It should echo the stored sample", func() {
				So(result, ShouldEqual, 0.02)
			})
		})
	})
}

func TestLeaveOneOutMedianObserve(t *testing.T) {
	Convey("Given a panel with peer samples", t, func() {
		panel := Panel{}
		leaveOneOut := NewLeaveOneOutMedian(&panel)

		_ = panel.Observe(nomagique.Scalar(1), nomagique.Scalar(0.02))
		_ = panel.Observe(nomagique.Scalar(2), nomagique.Scalar(0.04))
		_ = panel.Observe(nomagique.Scalar(3), nomagique.Scalar(0.06))

		Convey("It should return the peer median excluding the queried member", func() {
			So(nomagique.Scalar(1).Observe(leaveOneOut), ShouldEqual, 0.05)
		})
	})
}

func TestLeaveOneOutMedianNumber(t *testing.T) {
	Convey("Given a composed leave-one-out number", t, func() {
		panel := Panel{}
		leaveOneOut := NewLeaveOneOutMedian(&panel)

		macroNumber, err := nomagique.Number(leaveOneOut)

		So(err, ShouldBeNil)

		_ = panel.Observe(nomagique.Scalar(1), nomagique.Scalar(0.02))
		_ = panel.Observe(nomagique.Scalar(2), nomagique.Scalar(0.04))
		_ = panel.Observe(nomagique.Scalar(3), nomagique.Scalar(0.06))

		query := nomagique.Scalar(1)

		Convey("It should observe through the registered pipeline", func() {
			So(query.Observe(macroNumber), ShouldEqual, 0.05)
		})
	})
}

func BenchmarkLeaveOneOutMedianObserve(b *testing.B) {
	panel := Panel{}
	leaveOneOut := NewLeaveOneOutMedian(&panel)

	_ = panel.Observe(nomagique.Scalar(1), nomagique.Scalar(0.02))
	_ = panel.Observe(nomagique.Scalar(2), nomagique.Scalar(0.04))
	_ = panel.Observe(nomagique.Scalar(3), nomagique.Scalar(0.06))

	b.ReportAllocs()

	for b.Loop() {
		_ = nomagique.Scalar(1).Observe(leaveOneOut)
	}
}
