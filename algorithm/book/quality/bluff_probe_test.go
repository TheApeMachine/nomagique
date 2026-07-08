package quality

import (
	"testing"

	"github.com/theapemachine/nomagique/equation"
)

func TestBluffProbe(t *testing.T) {
	sample := NewSample(SampleConfig{})
	eq := equation.NewBookQuality()
	frames := []Level3Input{
		level3TouchAddFrame("B1", "B2", 100),
		level3TouchDeleteFrame("B1", 100),
	}

	bestBluff := 0.0

	for _, frame := range frames {
		input, ready, err := sample.MeasureLevel3(frame)

		if err != nil {
			t.Fatal(err)
		}

		if !ready {
			continue
		}

		output, measureErr := eq.Measure(input)

		if measureErr != nil {
			t.Fatal(measureErr)
		}

		bestBluff = max(bestBluff, output.BluffScore)
	}

	if bestBluff <= 0 {
		t.Fatal("expected positive bluff evidence")
	}
}
