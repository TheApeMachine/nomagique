package algorithm

import (
	"testing"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
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

	bestBluff := 0.0
	for _, frame := range frames {
		state := datura.Acquire("measurement", datura.APPJSON).
			WithRole("measurement").
			WithScope("update").
			WithPayload(frame)

		err := nomagique.RoundTripArtifact(state, pipeline)

		if err == nil {
			bestBluff = max(bestBluff, datura.Peek[float64](state, "output", "bluffScore"))
		}

		state.Release()
	}

	if bestBluff <= 0 {
		t.Fatal("expected positive bluff evidence")
	}
}
