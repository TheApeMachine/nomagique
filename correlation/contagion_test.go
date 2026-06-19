package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestMedianPairwiseAbsCorrelation(testingTB *testing.T) {
	Convey("Given proportional interval series", testingTB, func() {
		left := NewIntervalSeries(8)
		right := NewIntervalSeries(8)
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(float64(1_000), "sample").Poke(100.0, "paired")
		err := transport.NewFlipFlop(artifact, left)

		So(err, ShouldBeNil)

		artifact.Poke(float64(2_000), "sample").Poke(110.0, "paired")
		err = transport.NewFlipFlop(artifact, left)

		So(err, ShouldBeNil)

		artifact.Poke(float64(1_000), "sample").Poke(50.0, "paired")
		err = transport.NewFlipFlop(artifact, right)

		So(err, ShouldBeNil)

		artifact.Poke(float64(2_000), "sample").Poke(55.0, "paired")
		err = transport.NewFlipFlop(artifact, right)

		So(err, ShouldBeNil)

		Convey("It should return unit median correlation", func() {
			value := MedianPairwiseAbsCorrelation([]*IntervalSeries{left, right})
			So(value, ShouldAlmostEqual, 1, 1e-9)
		})
	})
}

func TestContagionObserve(testingTB *testing.T) {
	Convey("Given a contagion stage with fed window sets", testingTB, func() {
		first := NewWindowSet(8)
		second := NewWindowSet(8)
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(float64(1_000), "sample").Poke(100.0, "paired")
		err := transport.NewFlipFlop(artifact, first)

		So(err, ShouldBeNil)

		artifact.Poke(float64(2_000), "sample").Poke(110.0, "paired")
		err = transport.NewFlipFlop(artifact, first)

		So(err, ShouldBeNil)

		artifact.Poke(float64(1_000), "sample").Poke(50.0, "paired")
		err = transport.NewFlipFlop(artifact, second)

		So(err, ShouldBeNil)

		artifact.Poke(float64(2_000), "sample").Poke(55.0, "paired")
		err = transport.NewFlipFlop(artifact, second)

		So(err, ShouldBeNil)

		contagion := NewContagion(
			[]*WindowSet{first, second},
			TierWindows{Fast: 8, Medium: 8, Slow: 8},
			ContagionConfig{
				MinSamples:    1,
				MemberCap:     2,
				AdaptiveSigma: 2,
			},
		)

		trigger := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(trigger, contagion)

		So(err, ShouldBeNil)

		value := datura.Peek[float64](trigger, "output", "value")

		Convey("It should publish positive coupling for correlated tiers", func() {
			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkContagionObserve(testingTB *testing.B) {
	sets := make([]*WindowSet, 16)
	artifact := datura.Acquire("test", datura.APPJSON)

	for index := range sets {
		set := NewWindowSet(32)

		for step := range 32 {
			artifact.Poke(float64((step+1)*1_000), "sample").
				Poke(100+float64(index)+float64(step)*0.01, "paired")
			_ = transport.NewFlipFlop(artifact, set)
		}

		sets[index] = set
	}

	contagion := NewContagion(
		sets,
		TierWindows{Fast: 8, Medium: 16, Slow: 32},
		ContagionConfig{
			MinSamples: 8,
			MemberCap:  16,
		},
	)

	trigger := datura.Acquire("test", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(trigger, contagion)
	}
}
