package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
)

func TestPanelObserve(t *testing.T) {
	Convey("Given a Panel stage", t, func() {
		panel := NewPanel()

		Convey("It should echo the registered sample", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(1, "member").
				Poke(0.02, "sample")

			err := transport.NewFlipFlop(artifact, panel)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0.02)
		})
	})
}

func TestMedianPanelPeers(t *testing.T) {
	Convey("Given Panel then Median in a pipeline", t, func() {
		crossSection := nomagique.Number(NewPanel(), NewMedian())
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, member := range []struct {
			key   float64
			value float64
		}{
			{1, 0.02},
			{2, 0.04},
			{3, 0.06},
		} {
			artifact.Poke(member.key, "member").Poke(member.value, "sample")
			err := transport.NewFlipFlop(artifact, crossSection)

			So(err, ShouldBeNil)
		}

		artifact.Poke(1, "member").Poke(0.01, "sample")
		err := transport.NewFlipFlop(artifact, crossSection)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0.05)
	})
}
