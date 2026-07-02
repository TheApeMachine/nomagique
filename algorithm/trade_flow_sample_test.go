package algorithm

import (
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func tradeFrame(symbol, side string, price, quantity float64, timestamp int64) *datura.Artifact {
	payload := fmt.Sprintf(
		`{"channel":"trade","type":"update","data":[{"symbol":%q,"side":%q,"price":%g,"qty":%g,"timestamp":"2026-05-30T12:00:00Z"}]}`,
		symbol, side, price, quantity,
	)
	artifact := datura.Acquire("trade-flow-test", datura.APPJSON)
	artifact.WithPayload([]byte(payload))
	artifact.SetTimestamp(timestamp)

	return artifact
}

func TestTradeFlowSampleRead(testingTB *testing.T) {
	Convey("Given a sequence of buy trades", testingTB, func() {
		sample := NewTradeFlowSample(datura.Acquire("trade-flow-config", datura.APPJSON))
		flow := transport.NewPipeline(sample)
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC).UnixNano()

		var last *datura.Artifact
		var lastErr error

		for index := range 30 {
			frame := tradeFrame("BTC/USD", "buy", 100+float64(index)*0.01, 1, base+int64(index))
			lastErr = nomagique.RoundTripArtifact(frame, flow)
			last = frame
		}

		Convey("It should publish a flow feature batch after history warms", func() {
			So(lastErr, ShouldBeNil)
			So(datura.Peek[[]float64](last, "features"), ShouldNotBeNil)
			So(len(datura.Peek[[]float64](last, "features")), ShouldBeGreaterThan, 6)
		})
	})

	Convey("Given a non-trade channel frame", testingTB, func() {
		sample := NewTradeFlowSample(datura.Acquire("trade-flow-config", datura.APPJSON))
		payload := `{"channel":"ticker","type":"update","data":[{"symbol":"BTC/USD","last":100}]}`
		frame := datura.Acquire("trade-flow-ticker", datura.APPJSON)
		frame.WithPayload([]byte(payload))

		err := nomagique.RoundTripArtifact(frame, sample)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a single valid trade before history warms", testingTB, func() {
		sample := NewTradeFlowSample(datura.Acquire("trade-flow-config", datura.APPJSON))
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC).UnixNano()
		frame := tradeFrame("BTC/USD", "buy", 100, 1, base)

		err := nomagique.RoundTripArtifact(frame, sample)

		Convey("It should be nonfatal until trade history warms", func() {
			So(err, ShouldBeNil)
		})
	})
}

func TestTradeFlowSampleReadRowArtifacts(testingTB *testing.T) {
	Convey("Given row-level buy trade artifacts", testingTB, func() {
		sample := NewTradeFlowSample(datura.Acquire("trade-flow-config", datura.APPJSON))
		flow := equation.NewFlow(equation.FlowConfig())
		classifier := probability.NewClassifier(
			datura.Acquire("trade-flow-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs": []string{
					"absorption",
					"drive",
					"balance",
					"starvation",
				},
			}),
		)
		pipeline := transport.NewPipeline(sample, flow, classifier)
		var last *datura.Artifact
		var lastErr error

		for index := range 30 {
			frame := datura.Acquire("trade-flow-row-test", datura.APPJSON).
				WithRole("measurement").
				WithScope("BTC/USD").
				WithPayload([]byte(fmt.Sprintf(
					`{"symbol":"BTC/USD","side":"buy","price":%g,"qty":1}`,
					100+float64(index)*0.01,
				)))

			lastErr = nomagique.RoundTripArtifact(frame, pipeline)
			last = frame
		}

		Convey("It should classify flow without a Kraken envelope", func() {
			So(lastErr, ShouldBeNil)
			So(datura.Peek[string](last, "symbol"), ShouldEqual, "BTC/USD")
			So(datura.Peek[string](last, "root"), ShouldEqual, "output")
			So(datura.Peek[float64](last, "output", "drive"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](last, "output", "confidence"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkTradeFlowSampleRead(b *testing.B) {
	sample := NewTradeFlowSample(datura.Acquire("trade-flow-bench", datura.APPJSON))
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC).UnixNano()

	b.ReportAllocs()

	for b.Loop() {
		frame := tradeFrame("BTC/USD", "buy", 100, 1, base+int64(b.N))
		_ = nomagique.RoundTripArtifact(frame, sample)
	}
}
