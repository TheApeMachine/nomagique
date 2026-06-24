package algorithm

import (
	"fmt"
	"testing"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func bookFrameBytes(bidQty, askQty float64) []byte {
	return []byte(fmt.Sprintf(
		`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":%g}],"asks":[{"price":101,"qty":%g}]}]}`,
		bidQty, askQty,
	))
}

func replayBookQuality(
	frames [][]byte,
) (*datura.Artifact, float64, float64, float64) {
	encoder := NewBookQualitySample(bookQualitySampleConfig())
	bookQuality := equation.NewBookQuality(equation.BookQualityConfig())
	classifier := probability.NewClassifier(
		datura.Acquire("toxicity-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
			"inputs":    []string{"bluffScore", "vacuumScore", "supportScore"},
			"scoreRoot": "output",
		}),
	)
	pipeline := transport.NewPipeline(encoder, bookQuality, classifier)

	var (
		result    *datura.Artifact
		bestBluff float64
	)

	for _, frame := range frames {
		state := datura.Acquire("measurement", datura.APPJSON).
			WithRole("measurement").
			WithScope("update").
			WithPayload(frame)

		if transport.NewFlipFlop(state, pipeline) != nil {
			state.Release()
			continue
		}

		bluffScore := datura.Peek[float64](state, "output", "bluffScore")

		if bluffScore > bestBluff {
			if result != nil {
				result.Release()
			}

			result = state
			bestBluff = bluffScore

			continue
		}

		state.Release()
	}

	if result == nil {
		return nil, 0, 0, 0
	}

	confidence := datura.Peek[float64](result, "output", "confidence")
	category := datura.Peek[float64](result, "output", "category")
	bluffScore := datura.Peek[float64](result, "output", "bluffScore")

	return result, bluffScore, confidence, category
}

func bookQualityBluffReplayFrames() [][]byte {
	frames := make([][]byte, 0, 32)

	for range 12 {
		frames = append(frames, bookFrameBytes(50, 80))
	}

	for range 8 {
		frames = append(frames, bookFrameBytes(80, 80), bookFrameBytes(62, 80))
	}

	for range 4 {
		frames = append(frames, bookFrameBytes(110, 80), bookFrameBytes(45, 80))
	}

	return frames
}

func TestBookQualityBluffReplay(t *testing.T) {
	result, bluffScore, confidence, category := replayBookQuality(bookQualityBluffReplayFrames())

	if result == nil {
		t.Fatal("no bluff measurement")
	}

	defer result.Release()

	if bluffScore <= 0 || int(category) != 1 {
		t.Fatalf("bluff=%g category=%g confidence=%g", bluffScore, category, confidence)
	}
}
