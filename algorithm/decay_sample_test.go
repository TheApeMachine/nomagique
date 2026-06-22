package algorithm

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestDecaySample_Read(t *testing.T) {
	Convey("Given deteriorating bid depth on repeated book frames", t, func() {
		encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
		decay := equation.NewDecay(nil)
		classifier := probability.NewClassifier(
			datura.Acquire("exhaust-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs": []string{"mechanical", "fragile", "thermal", "reversal"},
			}),
		)
		pipeline := transport.NewPipeline(encoder, decay, classifier)

		quantities := []float64{20, 18, 16, 14, 12, 10, 8, 6, 4}

		var result *datura.Artifact

		for index, bidQty := range quantities {
			frame := []byte(fmt.Sprintf(
				`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":%g}],"asks":[{"price":101,"qty":10}]}]}`,
				bidQty,
			))
			state := datura.Acquire("measurement", datura.APPJSON).
				WithRole("measurement").
				WithScope("update").
				WithPayload(frame)

			err := transport.NewFlipFlop(state, pipeline)

			if index == len(quantities)-1 {
				So(err, ShouldBeNil)
			}

			if result != nil {
				result.Release()
			}

			result = state
		}

		Convey("It should emit calibrated decay output", func() {
			So(result, ShouldNotBeNil)
			So(datura.Peek[float64](result, "output", "value"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](result, "output", "confidence"), ShouldBeGreaterThan, 0.25)

			result.Release()
		})
	})
}

func TestDecaySample_ReadRejectsMissingSymbol(t *testing.T) {
	Convey("Given a book frame without symbol", t, func() {
		encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
		frame := []byte(`{"channel":"book","type":"update","data":[{"bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`)
		state := datura.Acquire("measurement", datura.APPJSON).WithPayload(frame)

		err := transport.NewFlipFlop(state, encoder)

		So(err, ShouldNotBeNil)
		state.Release()
	})
}

func BenchmarkDecaySample_Read(b *testing.B) {
	encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
	bookPayload := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`)
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = encoder.Write(bookPayload)
		_, _ = encoder.Read(frame)
	}
}
