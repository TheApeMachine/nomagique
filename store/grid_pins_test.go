package store

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRegionMetricPinCardinality(t *testing.T) {
	Convey("each confluence region has exactly two pinned metrics", t, func() {
		counts := make([]int, RegionCount)
		for _, metrics := range regionMetricPins {
			for _, regions := range metrics {
				for _, key := range regions {
					if key < RegionCount {
						counts[key]++
					}
				}
			}
		}

		for region := uint8(0); region < RegionCount; region++ {
			So(counts[region], ShouldEqual, 2)
		}
	})
}
