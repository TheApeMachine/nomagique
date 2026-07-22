package flow_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/algorithm/book/flow"
)

/*
TestToxicPenalty proves atomic same-side replacement is not mistaken for a
liquidity withdrawal while unmatched touch cancellation remains observable.
*/
func TestToxicPenalty(t *testing.T) {
	Convey("Given touch liquidity changes from one atomic book update", t, func() {
		Convey("A complete same-side replacement should not be toxic", func() {
			So(flow.ToxicPenalty(20_000, 20_000, 20_000), ShouldEqual, 0)
		})

		Convey("An unmatched withdrawal should retain its depth-relative share", func() {
			So(flow.ToxicPenalty(10_000, 0, 20_000), ShouldEqual, 1.0/3.0)
		})

		Convey("A withdrawal with no remaining touch should be fully toxic", func() {
			So(flow.ToxicPenalty(10_000, 0, 0), ShouldEqual, 1)
		})
	})
}

/*
BenchmarkToxicPenalty measures cancellation evidence on the sampler's book
update hot path.
*/
func BenchmarkToxicPenalty(b *testing.B) {
	for b.Loop() {
		flow.ToxicPenalty(10_000, 2_500, 20_000)
	}
}
