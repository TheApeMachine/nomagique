package equation

import (
	"testing"
	"time"
)

func TestLogReturnZScoreMeasure(t *testing.T) {
	stage, err := NewLogReturnZScore(LogReturnZScoreConfig{
		ReturnLag:    1,
		LongWindow:   5,
		PositiveOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Unix(0, 1)
	var output LogReturnZScoreOutput

	for _, price := range []float64{100, 101, 102, 103, 104, 200} {
		output, err = stage.Measure(LogReturnZScoreSample{
			Series: "BTC/USD",
			Price:  price,
			At:     timestamp,
		})
		if err != nil {
			t.Fatal(err)
		}

		timestamp = timestamp.Add(time.Second)
	}

	if !output.Ready {
		t.Fatal("expected ready output")
	}

	if output.Value <= 0 {
		t.Fatalf("precursor = %f, want positive", output.Value)
	}
}

func TestLogReturnZScoreRequiresReturnLag(t *testing.T) {
	_, err := NewLogReturnZScore(LogReturnZScoreConfig{})
	if err == nil {
		t.Fatal("expected missing return lag error")
	}
}

func BenchmarkLogReturnZScoreMeasure(testingTB *testing.B) {
	stage, err := NewLogReturnZScore(LogReturnZScoreConfig{
		ReturnLag:    1,
		LongWindow:   5,
		PositiveOnly: true,
	})
	if err != nil {
		testingTB.Fatal(err)
	}

	timestamp := time.Unix(0, 1)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := stage.Measure(LogReturnZScoreSample{
			Series: "BTC/USD",
			Price:  105,
			At:     timestamp,
		}); err != nil {
			testingTB.Fatal(err)
		}

		timestamp = timestamp.Add(time.Second)
	}
}
