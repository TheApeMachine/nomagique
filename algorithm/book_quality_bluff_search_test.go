package algorithm

import (
	"testing"

	"github.com/theapemachine/nomagique/equation"
)

func level3TouchAddFrame(
	orderID string,
	reserveID string,
	quantity float64,
) BookQualityLevel3Input {
	return BookQualityLevel3Input{
		Symbol: "BTC/USD",
		Bids: []BookQualityOrderEvent{
			{Event: "add", OrderID: orderID, Price: 100, Quantity: quantity},
			{Event: "add", OrderID: reserveID, Price: 100, Quantity: quantity / 5},
		},
		Asks: []BookQualityOrderEvent{
			{Event: "add", OrderID: "A1", Price: 101, Quantity: quantity},
		},
	}
}

func level3TouchDeleteFrame(orderID string, quantity float64) BookQualityLevel3Input {
	return BookQualityLevel3Input{
		Symbol: "BTC/USD",
		Bids: []BookQualityOrderEvent{
			{Event: "delete", OrderID: orderID, Price: 100, Quantity: quantity},
		},
	}
}

func replayBookQuality(
	frames []BookQualityLevel3Input,
) (equation.BookQualityOutput, bool, error) {
	sample := NewBookQualitySample(bookQualitySampleConfig())
	bookQuality := equation.NewBookQuality()
	bestOutput := equation.BookQualityOutput{}
	seen := false

	for _, frame := range frames {
		input, ready, err := sample.MeasureLevel3(frame)

		if err != nil {
			return equation.BookQualityOutput{}, false, err
		}

		if !ready {
			continue
		}

		output, measureErr := bookQuality.Measure(input)

		if measureErr != nil {
			return equation.BookQualityOutput{}, false, measureErr
		}

		if output.BluffScore <= bestOutput.BluffScore {
			continue
		}

		bestOutput = output
		seen = true
	}

	return bestOutput, seen, nil
}

func bookQualityBluffReplayFrames() []BookQualityLevel3Input {
	return []BookQualityLevel3Input{
		level3TouchAddFrame("B1", "B2", 100),
		level3TouchDeleteFrame("B1", 100),
	}
}

func TestBookQualityBluffReplay(t *testing.T) {
	output, seen, err := replayBookQuality(bookQualityBluffReplayFrames())

	if err != nil {
		t.Fatal(err)
	}

	if !seen {
		t.Fatal("no bluff measurement")
	}

	if output.BluffScore <= 0 || int(output.Category) != 1 {
		t.Fatalf("bluff=%g category=%g strength=%g", output.BluffScore, output.Category, output.Strength)
	}
}
