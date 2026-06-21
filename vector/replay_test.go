package vector

import (
	"io"
	"testing"

	"github.com/theapemachine/datura"
)

func TestFeatureExtractorReplayRead(t *testing.T) {
	extractor := NewFeatureExtractor(featureExtractorSchema())
	inbound := datura.Acquire("test", datura.APPJSON).WithPayload(featureExtractorPayloadFixture)

	if _, err := io.Copy(extractor, inbound); err != nil {
		t.Fatal(err)
	}

	buffer := make([]byte, 65536)

	for tick := range 200000 {
		if _, err := extractor.Read(buffer); err != nil && err != io.EOF {
			t.Fatalf("tick %d: %v", tick, err)
		}
	}
}
