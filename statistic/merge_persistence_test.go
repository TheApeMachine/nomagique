package statistic

import (
	"testing"

	"github.com/theapemachine/datura"
)

func TestConfigMergePersistence(t *testing.T) {
	config := datura.Acquire("merge-persistence", datura.APPJSON)

	for index := range 200 {
		history := datura.Peek[[]float64](config, "history")
		history = append(history, float64(index))
		config.Merge("history", history)
	}

	history := datura.Peek[[]float64](config, "history")

	if len(history) != 200 {
		t.Fatalf("history length = %d, want 200", len(history))
	}
}
