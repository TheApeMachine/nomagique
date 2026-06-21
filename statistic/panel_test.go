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
		panel := NewPanel(datura.Acquire("panel-config", datura.APPJSON))

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
		panelConfig := datura.Acquire("panel-config", datura.APPJSON)
		medianConfig := datura.Acquire("median-config", datura.APPJSON)
		crossSection := nomagique.Number(NewPanel(panelConfig), NewMedian(medianConfig))
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
