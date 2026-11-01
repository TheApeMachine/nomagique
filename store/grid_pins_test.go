package store

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRegionMetricPinsInRange(t *testing.T) {
	Convey("every pin targets a valid confluence region", t, func() {
		for source, metrics := range regionMetricPins {
			for label, regions := range metrics {
				So(len(regions), ShouldBeGreaterThan, 0)

				for _, key := range regions {
					So(key, ShouldBeLessThan, RegionCount)
					_ = source
					_ = label
				}
			}
		}
	})
}
