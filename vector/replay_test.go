package vector

import (
	"testing"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

// TestFeatureExtractorReplayRead drives the extractor the way a real pipeline
// does: one stage instance, a fresh datapoint flipped through it per tick, the
// computed features read back off the same artifact payload.
func TestFeatureExtractorReplayRead(t *testing.T) {
	extractor := NewFeatureExtractor(featureExtractorSchema())

	for tick := range 1000 {
		datapoint := datura.Acquire("test", datura.APPJSON).WithPayload(featureExtractorPayloadFixture)

		if err := transport.NewFlipFlop(datapoint, extractor); err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}

		if datura.Peek[string](datapoint, "root") != "features" {
			t.Fatalf("tick %d: expected root features", tick)
		}

		features := datura.Peek[[]float64](datapoint, "features")

		if len(features) != 6 {
			t.Fatalf("tick %d: expected 6 features, got %d", tick, len(features))
		}

		datapoint.Release()
	}
}
