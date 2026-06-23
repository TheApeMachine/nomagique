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
		state.Poke([]string{"volume", "last", "change_pct"}, "featureInputs")
		state.MergeOutput("rvol", 3.0)
		state.Poke("output", "root")

		Convey("It should read raw columns by schema key", func() {
			last, err := FeatureColumn(state, "last")
			So(err, ShouldBeNil)
			So(last, ShouldEqual, 42.5)

			volume, err := FeatureColumn(state, "volume")
			So(err, ShouldBeNil)
			So(volume, ShouldEqual, 1000)
		})

		Convey("It should error on missing keys", func() {
			_, err := FeatureColumn(state, "missing")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestFeatureSnapshotRestore(t *testing.T) {
	Convey("Given a feature snapshot", t, func() {
		source := datura.Acquire("feature-snapshot-source", datura.APPJSON)
		source.Merge("features", []float64{1, 2})
		source.Poke([]string{"bid", "ask"}, "featureInputs")

		snapshot := SnapshotFeatures(source)
		target := datura.Acquire("feature-snapshot-target", datura.APPJSON)
		snapshot.Restore(target)

		Convey("It should restore extracted columns", func() {
			bid, err := FeatureColumn(target, "bid")
			So(err, ShouldBeNil)
			So(bid, ShouldEqual, 1)

			ask, err := FeatureColumn(target, "ask")
			So(err, ShouldBeNil)
			So(ask, ShouldEqual, 2)
		})
	})
}
