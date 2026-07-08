package quality

import (
	"testing"

	"github.com/theapemachine/nomagique/equation"
)

func level3TouchAddFrame(
	orderID string,
	reserveID string,
	quantity float64,
) Level3Input {
	return Level3Input{
		Symbol: "BTC/USD",
		Bids: []OrderEvent{
			{Event: "add", OrderID: orderID, Price: 100, Quantity: quantity},
			{Event: "add", OrderID: reserveID, Price: 100, Quantity: quantity / 5},
		},
		Asks: []OrderEvent{
			{Event: "add", OrderID: "A1", Price: 101, Quantity: quantity},
		},
	}
}

func level3TouchDeleteFrame(orderID string, quantity float64) Level3Input {
	return Level3Input{
		Symbol: "BTC/USD",
		Bids: []OrderEvent{
			{Event: "delete", OrderID: orderID, Price: 100, Quantity: quantity},
		},
	}
}

func replay(
	frames []Level3Input,
) (equation.BookQualityOutput, bool, error) {
	sample := NewSample(SampleConfig{})
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

func BluffReplayFrames() []Level3Input {
	return []Level3Input{
		level3TouchAddFrame("B1", "B2", 100),
		level3TouchDeleteFrame("B1", 100),
	}
}

func TestBluffReplay(t *testing.T) {
	output, seen, err := replay(BluffReplayFrames())

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
