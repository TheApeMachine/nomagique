package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestMoveBaselineRead(testingTB *testing.T) {
	Convey("Given a warmed move baseline", testingTB, func() {
		config := datura.Acquire("move-baseline-config", datura.APPJSON).
			Poke(float64(anchorMoveMinObs), "minObs").
			Poke(float64(256), "pathCap").
			Poke("wire", "root").
			Poke("sample", "sampleKey")
		baseline := NewMoveBaseline(config)
		wire := datura.Acquire("move-baseline-wire", datura.APPJSON).
			Poke("wire", "root").
			Poke([]string{"sample"}, "inputs")

		for index := range anchorMoveMinObs {
			wire.Poke(0.0001+float64(index%2)*0.00005, "wire", "sample")
			err := nomagique.RoundTripArtifact(wire, baseline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](wire, "output", "ready"), ShouldEqual, 0)
		}

		Convey("It should classify a flat reading as stall with unit margin", func() {
			wire.Poke(0.00001, "wire", "sample")
			err := nomagique.RoundTripArtifact(wire, baseline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](wire, "output", "ready"), ShouldEqual, 1)
			So(datura.Peek[float64](wire, "output", "moved"), ShouldEqual, 0)
			margin := datura.Peek[float64](wire, "output", "stallMargin")
			So(margin, ShouldBeGreaterThan, 0)
			So(margin, ShouldBeLessThanOrEqualTo, 1)
		})
	})
}

const (
	anchorMoveMinObs = 12
)

var barInterval = 5 * time.Minute
