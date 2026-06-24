package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
)

func TestPanelRead(t *testing.T) {
	Convey("Given a Panel stage", t, func() {
		config := datura.Acquire("panel-config", datura.APPJSON).
			Poke("member", "memberKey").
			Poke("sample", "sampleKey")
		panel := NewPanel(config)

		Convey("It should echo the registered sample", func() {
			artifact := PanelWire(datura.Acquire("test", datura.APPJSON), 1, 0.02)
			err := transport.NewFlipFlop(artifact, panel)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0.02)
		})
	})
}

func TestMedianPanelPeers(t *testing.T) {
	Convey("Given Panel then Median in a pipeline", t, func() {
		panelConfig := datura.Acquire("panel-config", datura.APPJSON).
			Poke("member", "memberKey").
			Poke("sample", "sampleKey")
		medianConfig := datura.Acquire("median-config", datura.APPJSON).
			Poke("member", "memberKey")
		panel := NewPanel(panelConfig)
		crossSection := nomagique.Number(panel, NewMedian(medianConfig))

		for _, member := range []struct {
			key   float64
			value float64
		}{
			{1, 0.02},
			{2, 0.04},
			{3, 0.06},
		} {
			artifact := PanelWire(datura.Acquire("test", datura.APPJSON), member.key, member.value)
			err := transport.NewFlipFlop(artifact, panel)

			So(err, ShouldBeNil)
			artifact.Release()
		}

		artifact := PanelWire(datura.Acquire("test", datura.APPJSON), 1, 0.01)
		err := transport.NewFlipFlop(artifact, crossSection)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0.05)
		artifact.Release()
	})
}
