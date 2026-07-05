package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFeatureColumn(t *testing.T) {
	Convey("Given extracted features with output root", t, func() {
		snapshot := NewFeatureSnapshot(
			[]string{"volume", "last", "change_pct"},
			[]float64{1000, 42.5, 0.1},
		)

		Convey("It should read raw columns by schema key", func() {
			last, err := FeatureColumn(snapshot, "last")
			So(err, ShouldBeNil)
			So(last, ShouldEqual, 42.5)

			volume, err := FeatureColumn(snapshot, "volume")
			So(err, ShouldBeNil)
			So(volume, ShouldEqual, 1000)
		})

		Convey("It should error on missing keys", func() {
			_, err := FeatureColumn(snapshot, "missing")
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkFeatureSnapshotValue(b *testing.B) {
	snapshot := NewFeatureSnapshot(
		[]string{"bid", "ask", "last"},
		[]float64{1, 2, 3},
	)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = snapshot.Value("ask")
	}
}
