package algorithm

import (
	"fmt"
	"io"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func cohortSampleConfig() *datura.Artifact {
	return datura.Acquire("cohort-sample-config", datura.APPJSON).WithAttributes(datura.Map[any]{
		"channel":     "ticker",
		"root":        "data",
		"symbolInput": "symbol",
		"priceInput":  "last",
	})
}

func cohortSampleTicker(symbol string, price float64, timestamp int64) *datura.Artifact {
	payload := fmt.Sprintf(
		`{"channel":"ticker","type":"update","data":[{"symbol":%q,"last":%g}]}`,
		symbol,
		price,
	)
	artifact := datura.Acquire("cohort-sample-frame", datura.APPJSON)
	artifact.WithPayload([]byte(payload))
	artifact.SetTimestamp(timestamp)

	return artifact
}

func TestCohortSampleEmitsFeatureSchema(testingTB *testing.T) {
	Convey("Given ticker prices for a live cohort", testingTB, func() {
		sample := NewCohortSample(cohortSampleConfig())
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		symbols := []string{"ALPHA/USD", "BETA/USD", "GAMMA/USD"}
		var result *datura.Artifact

		for tick := range 4 {
			for symbolIndex, symbol := range symbols {
				frame := cohortSampleTicker(
					symbol,
					100+float64(tick*symbolIndex)+float64(tick),
					base.Add(time.Duration(tick)*time.Second).UnixNano(),
				)

				if err := runCohortSample(frame, sample); err == nil {
					result = frame
				}
			}
		}

		Convey("It should write cohort features onto the artifact", func() {
			So(result, ShouldNotBeNil)
			So(datura.Peek[bool](result, "output", "ready"), ShouldBeTrue)
			So(datura.Peek[string](result, "root"), ShouldEqual, "features")
			So(datura.Peek[[]string](result, "inputs"), ShouldResemble, equation.CohortInputKeys)
			So(datura.Peek[float64](result, "features", 0), ShouldBeGreaterThanOrEqualTo, 2)
			So(datura.Peek[float64](result, "features", 5), ShouldBeGreaterThan, 0)
		})
	})
}

func TestCohortSampleColdStartReturnsEOF(testingTB *testing.T) {
	Convey("Given the first ticker price for a symbol", testingTB, func() {
		sample := NewCohortSample(cohortSampleConfig())
		frame := cohortSampleTicker(
			"ALPHA/USD",
			100,
			time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC).UnixNano(),
		)

		err := runCohortSample(frame, sample)

		Convey("It should stage the sample as not ready, not validation-failed", func() {
			So(err, ShouldEqual, io.EOF)
			So(datura.Peek[bool](frame, "output", "ready"), ShouldBeFalse)
			So(len(datura.Peek[[]float64](frame, "features")), ShouldEqual, 0)
		})
	})
}

func TestCohortSampleHistoryIsBounded(testingTB *testing.T) {
	Convey("Given a configured per-symbol history cap", testingTB, func() {
		historyCap := 8
		config := cohortSampleConfig().Poke(float64(historyCap), "historyCap")
		sample := NewCohortSample(config)
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
		symbols := []string{"ALPHA/USD", "BETA/USD", "GAMMA/USD"}

		for tick := range 32 {
			for symbolIndex, symbol := range symbols {
				frame := cohortSampleTicker(
					symbol,
					100+float64(tick)+float64(symbolIndex),
					base.Add(time.Duration(tick)*time.Second).UnixNano(),
				)
				_ = runCohortSample(frame, sample)
				frame.Release()
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
	sample := NewCohortSample(cohortSampleConfig())
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	symbols := []string{"ALPHA/USD", "BETA/USD", "GAMMA/USD", "DELTA/USD"}

	for tick := range 8 {
		for symbolIndex, symbol := range symbols {
			frame := cohortSampleTicker(
				symbol,
				100+float64(tick)+float64(symbolIndex),
				base.Add(time.Duration(tick)*time.Second).UnixNano(),
			)
			_ = runCohortSample(frame, sample)
			frame.Release()
		}
	}

	testingTB.ReportAllocs()
	testingTB.ResetTimer()

	for tick := range testingTB.N {
		symbol := symbols[tick%len(symbols)]
		frame := cohortSampleTicker(
			symbol,
			100+float64(tick%97),
			base.Add(time.Duration(tick)*time.Second).UnixNano(),
		)
		_ = runCohortSample(frame, sample)
		frame.Release()
	}
}

func runCohortSample(frame *datura.Artifact, sample *CohortSample) error {
	wire := frame.Pack()

	if len(wire) == 0 {
		return io.EOF
	}

	if _, err := sample.Write(wire); err != nil {
		return err
	}

	chunk := make([]byte, 262144)
	readCount, err := sample.Read(chunk)

	if err != nil && (err != io.EOF || readCount == 0) {
		return err
	}

	_, err = frame.Unpack(chunk[:readCount])

	return err
}
