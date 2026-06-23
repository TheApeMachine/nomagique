package algorithm

import (
	"fmt"
	"io"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
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
			lastErr = transport.NewFlipFlop(frame, flow)
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

		err := transport.NewFlipFlop(frame, sample)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a single trade before history warms", testingTB, func() {
		sample := NewTradeFlowSample(datura.Acquire("trade-flow-config", datura.APPJSON))
		base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC).UnixNano()
		frame := tradeFrame("BTC/USD", "buy", 100, 1, base)

		err := transport.NewFlipFlop(frame, sample)

		Convey("It should wait for more trade history without error", func() {
			So(err, ShouldEqual, io.EOF)
		})
	})
}

func BenchmarkTradeFlowSampleRead(b *testing.B) {
	sample := NewTradeFlowSample(datura.Acquire("trade-flow-bench", datura.APPJSON))
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC).UnixNano()

	b.ReportAllocs()

	for b.Loop() {
		frame := tradeFrame("BTC/USD", "buy", 100, 1, base+int64(b.N))
		_ = transport.NewFlipFlop(frame, sample)
	}
}
