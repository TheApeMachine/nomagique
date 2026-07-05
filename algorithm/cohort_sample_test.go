package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func cohortSampleTicker(symbol string, price float64, timestamp time.Time) CohortSampleInput {
	return CohortSampleInput{
		Symbol: symbol,
		Price:  price,
		At:     timestamp,
	}
}

func TestCohortSampleEmitsFeatureSchema(testingTB *testing.T) {
	Convey("Given ticker prices for a live cohort", testingTB, func() {
		sample := NewCohortSample()
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		symbols := []string{"ALPHA/USD", "BETA/USD", "GAMMA/USD"}
		var result equation.FeatureFrame
		ready := false

		for tick := range 4 {
			for symbolIndex, symbol := range symbols {
				frame, frameReady, err := sample.Measure(cohortSampleTicker(
					symbol,
					100+float64(tick*symbolIndex)+float64(tick),
					base.Add(time.Duration(tick)*time.Second),
				))

				So(err, ShouldBeNil)

				if frameReady {
					result = frame
					ready = true
				}
			}
		}

		Convey("It should emit cohort features with schema", func() {
			So(ready, ShouldBeTrue)
			So(result.Root, ShouldEqual, "features")
			So(result.Inputs, ShouldResemble, equation.CohortInputKeys)
			So(result.Features[0], ShouldBeGreaterThanOrEqualTo, 2)
			So(result.Features[5], ShouldBeGreaterThan, 0)
		})
	})
}

func TestCohortSampleColdStartReturnsEOF(testingTB *testing.T) {
	Convey("Given the first ticker price for a symbol", testingTB, func() {
		sample := NewCohortSample()
		frame := cohortSampleTicker(
			"ALPHA/USD",
			100,
			time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		)

		output, ready, err := sample.Measure(frame)

		Convey("It should stage the sample as not ready, not validation-failed", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeFalse)
			So(len(output.Features), ShouldEqual, 0)
		})
	})
}

func TestCohortSampleHistoryIsBounded(testingTB *testing.T) {
	Convey("Given a configured per-symbol history cap", testingTB, func() {
		historyCap := 8
		sample := NewCohortSample(CohortSampleConfig{HistoryCap: historyCap})
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		symbols := []string{"ALPHA/USD", "BETA/USD", "GAMMA/USD"}

		for tick := range 32 {
			for symbolIndex, symbol := range symbols {
				_, _, _ = sample.Measure(cohortSampleTicker(
					symbol,
					100+float64(tick)+float64(symbolIndex),
					base.Add(time.Duration(tick)*time.Second),
				))
			}
		}

		Convey("It should retain no more than the configured cap per symbol", func() {
			for _, symbol := range symbols {
				symbolState := sample.symbols[symbol]
				So(symbolState, ShouldNotBeNil)
				So(len(symbolState.returns), ShouldBeLessThanOrEqualTo, historyCap)
				So(len(symbolState.times), ShouldBeLessThanOrEqualTo, historyCap)
			}
		})
	})
}

func BenchmarkCohortSample(testingTB *testing.B) {
	sample := NewCohortSample()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	symbols := []string{"ALPHA/USD", "BETA/USD", "GAMMA/USD", "DELTA/USD"}

	for tick := range 8 {
		for symbolIndex, symbol := range symbols {
			_, _, _ = sample.Measure(cohortSampleTicker(
				symbol,
				100+float64(tick)+float64(symbolIndex),
				base.Add(time.Duration(tick)*time.Second),
			))
		}
	}

	testingTB.ReportAllocs()
	testingTB.ResetTimer()

	for tick := range testingTB.N {
		symbol := symbols[tick%len(symbols)]
		_, _, _ = sample.Measure(cohortSampleTicker(
			symbol,
			100+float64(tick%97),
			base.Add(time.Duration(tick)*time.Second),
		))
	}
}
