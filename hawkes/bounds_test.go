package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSoftplus(t *testing.T) {
	Convey("Given softplus and its inverse", t, func() {
		value := softplus(0.5)
		roundTrip := inverseSoftplus(value)

		Convey("It should round-trip in log space", func() {
			So(roundTrip, ShouldAlmostEqual, 0.5, 1e-9)
		})
	})
}

func TestLogParamBounds_encodeDecode(t *testing.T) {
	Convey("Given fit context bounds", t, func() {
		context := FitContext{
			SpanSec:        10,
			TotalEvents:    20,
			EventsX:        10,
			EventsY:        10,
			MedianGapSec:   0.5,
			BetaCandidates: []float64{0.5, 1, 2},
			BranchFloor:    0.01,
			BranchCeiling:  0.9,
			LocalScales:    []float64{0.5},
		}
		bounds, err := context.logParamBounds()
		So(err, ShouldBeNil)
		start := bounds.encode([bivariateParamCount]float64{
			0, 0, 0, -2, -3, -3, -2,
		})
		decoded := bounds.decode(start)

		Convey("It should stay inside configured bounds", func() {
			for index := range decoded {
				So(decoded[index], ShouldBeGreaterThanOrEqualTo, bounds.lower[index])
				So(decoded[index], ShouldBeLessThanOrEqualTo, bounds.upper[index])
			}
		})
	})
}
