package algorithm

import (
	"fmt"
	"testing"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestBookQualityBluffProbe(t *testing.T) {
	encoder := NewBookQualitySample(bookQualitySampleConfig())
	bookQuality := equation.NewBookQuality(equation.BookQualityConfig())
	classifier := probability.NewClassifier(
		datura.Acquire("toxicity-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
			"inputs":    []string{"bluffScore", "vacuumScore", "supportScore"},
			"scoreRoot": "output",
		}),
	)
	pipeline := transport.NewPipeline(encoder, bookQuality, classifier)

	frames := [][]byte{
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":100}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":2}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":120}],"asks":[{"price":101,"qty":10}]}]}`),
		[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":1}],"asks":[{"price":101,"qty":10}]}]}`),
	}

	for index, frame := range frames {
		state := datura.Acquire("measurement", datura.APPJSON).
			WithRole("measurement").
			WithScope("update").
			WithPayload(frame)

		err := transport.NewFlipFlop(state, pipeline)

		fmt.Printf(
			"frame=%d err=%v near=%g bluffStr=%g churnGate=%g bluff=%g vacuum=%g support=%g category=%g confidence=%g\n",
			index, err,
			datura.Peek[float64](state, "features", 6),
			datura.Peek[float64](state, "features", 7),
			datura.Peek[float64](state, "features", 9),
			datura.Peek[float64](state, "output", "bluffScore"),
			datura.Peek[float64](state, "output", "vacuumScore"),
			datura.Peek[float64](state, "output", "supportScore"),
			datura.Peek[float64](state, "output", "category"),
			datura.Peek[float64](state, "output", "confidence"),
		)

		state.Release()
	}
}
