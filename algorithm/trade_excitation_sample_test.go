package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
)

func TestTradeExcitationSampleRead(testingTB *testing.T) {
	Convey("Given alternating buy and sell trades", testingTB, func() {
		sample := NewTradeExcitationSample(datura.Acquire("trade-excitation-config", datura.APPJSON))
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

		var last *datura.Artifact

		for index := range 32 {
			side := "buy"

			if index%2 == 0 {
				side = "sell"
			}

			frame := tradeFrame(
				"ALT/EUR",
				side,
				1,
				1,
				base.Add(time.Duration(index)*100*time.Millisecond).UnixNano(),
			)
			So(transport.NewFlipFlop(frame, sample), ShouldBeNil)
			last = frame
		}

		Convey("It should publish an excitation feature batch", func() {
			features := datura.Peek[[]float64](last, "features")
			So(len(features), ShouldBeGreaterThan, 8)
		})
	})

	Convey("Given a warmed excitation pipeline", testingTB, func() {
		excitation := NewExcitation(datura.Acquire("excitation-config", datura.APPJSON))
		pipeline := nomagique.Number(
			NewTradeExcitationSample(datura.Acquire("trade-excitation-config", datura.APPJSON)),
			excitation,
		)
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

		var last *datura.Artifact

		for index := range 128 {
			side := "buy"

			if index%2 == 0 {
				side = "sell"
			}

			frame := tradeFrame(
				"ALT/EUR",
				side,
				1,
				1,
				base.Add(time.Duration(index)*100*time.Millisecond).UnixNano(),
			)
			So(transport.NewFlipFlop(frame, pipeline), ShouldBeNil)
			last = frame
		}

		for range 4 {
			So(transport.NewFlipFlop(last, pipeline), ShouldBeNil)
		}

		Convey("It should publish excitation thermal scores", func() {
			So(excitation.Outcome().Strength, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given sample output wired directly into excitation", testingTB, func() {
		sample := NewTradeExcitationSample(datura.Acquire("trade-excitation-config", datura.APPJSON))
		excitation := NewExcitation(datura.Acquire("excitation-config", datura.APPJSON))
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

		var last *datura.Artifact

		for index := range 128 {
			side := "buy"

			if index%2 == 0 {
				side = "sell"
			}

			frame := tradeFrame(
				"ALT/EUR",
				side,
				1,
				1,
				base.Add(time.Duration(index)*100*time.Millisecond).UnixNano(),
			)
			So(transport.NewFlipFlop(frame, sample), ShouldBeNil)
			last = frame
		}

		batch := datura.Peek[[]float64](last, "features")
		inbound := daturaBurstArtifact("ALT/EUR", batch)

		for range 4 {
			So(transport.NewFlipFlop(inbound, excitation), ShouldBeNil)
		}

		Convey("It should publish excitation thermal scores", func() {
			So(excitation.Outcome().Strength, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkTradeExcitationSampleRead(b *testing.B) {
	sample := NewTradeExcitationSample(datura.Acquire("trade-excitation-bench", datura.APPJSON))
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

	b.ReportAllocs()

	for b.Loop() {
		frame := tradeFrame("ALT/EUR", "buy", 1, 1, base.Add(time.Duration(b.N)*time.Millisecond).UnixNano())
		_ = transport.NewFlipFlop(frame, sample)
	}
}
