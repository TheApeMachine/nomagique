package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestMultiverse_Observe(testingTB *testing.T) {
	Convey("Given correlated window sets", testingTB, func() {
		first := NewWindowSet(16)
		second := NewWindowSet(16)
		artifact := datura.Acquire("test", datura.APPJSON)

		for step := range 16 {
			epoch := float64((step + 1) * 1_000)
			artifact.Poke(epoch, "sample").Poke(100+float64(step)*0.1, "paired")
			err := transport.NewFlipFlop(artifact, first)

			So(err, ShouldBeNil)

			artifact.Poke(epoch, "sample").Poke(50+float64(step)*0.05, "paired")
			err = transport.NewFlipFlop(artifact, second)

			So(err, ShouldBeNil)
		}

		multiverse := NewMultiverse(
			[]*WindowSet{first, second},
			TierWindows{Fast: 4, Medium: 8, Slow: 16},
			ContagionConfig{MinSamples: 2, MemberCap: 2, AdaptiveSigma: 2},
		)

		trigger := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(trigger, multiverse)

		So(err, ShouldBeNil)

		coupling := datura.Peek[float64](trigger, "output", "value")
		readings := multiverse.TierReadings()

		Convey("It should publish positive coupling", func() {
			So(coupling, ShouldBeGreaterThan, 0)
			So(readings.Fast, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkMultiverse_Observe(testingTB *testing.B) {
	sets := make([]*WindowSet, 8)
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

	multiverse := NewMultiverse(
		sets,
		TierWindows{Fast: 8, Medium: 16, Slow: 32},
		ContagionConfig{MinSamples: 4, MemberCap: 8},
	)

	trigger := datura.Acquire("test", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(trigger, multiverse)
	}
}
