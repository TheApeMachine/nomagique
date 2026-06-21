package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestFeatureColumn(t *testing.T) {
	Convey("Given extracted features with output root", t, func() {
		state := datura.Acquire("feature-column-test", datura.APPJSON)
		state.Merge("features", []float64{1000, 42.5, 0.1})
		state.Merge("inputs", []string{"volume", "last", "change_pct"})
		state.MergeOutput("rvol", 3.0)
		state.Merge("root", "output")

		Convey("It should read raw columns by schema key", func() {
			So(FeatureColumn(state, "last"), ShouldEqual, 42.5)
			So(FeatureColumn(state, "volume"), ShouldEqual, 1000)
		})
	})
}

func TestFeatureSnapshotRestore(t *testing.T) {
	Convey("Given a feature snapshot", t, func() {
		source := datura.Acquire("feature-snapshot-source", datura.APPJSON)
		source.Merge("features", []float64{1, 2})
		source.Merge("inputs", []string{"bid", "ask"})

		snapshot := SnapshotFeatures(source)
		target := datura.Acquire("feature-snapshot-target", datura.APPJSON)
		snapshot.Restore(target)

		Convey("It should restore extracted columns", func() {
			So(FeatureColumn(target, "bid"), ShouldEqual, 1)
			So(FeatureColumn(target, "ask"), ShouldEqual, 2)
		})
	})
}
