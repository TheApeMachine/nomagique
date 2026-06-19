package vector

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
SpreadSample derives a relative spread sample from two feature slots.
*/
type SpreadSample struct {
	config *datura.Artifact
	staged *datura.Artifact
}

/*
NewSpreadSample returns a spread sample stage configured on the artifact.
*/
func NewSpreadSample(config *datura.Artifact) *SpreadSample {
	return &SpreadSample{
		config: config,
		staged: datura.Acquire("spread-sample", datura.APPJSON),
	}
}

func (spreadSample *SpreadSample) Write(payload []byte) (int, error) {
	return spreadSample.staged.Write(payload)
}

func (spreadSample *SpreadSample) Read(payload []byte) (int, error) {
	rootKey := datura.Peek[string](spreadSample.staged, "root")
	channelKeys := datura.Peek[[]string](spreadSample.staged, "inputs")
	sourceKeys := datura.Peek[[]string](spreadSample.config, "inputs", "spread", "inputs")

	if rootKey == "" || len(channelKeys) == 0 || len(sourceKeys) < 2 {
		return spreadSample.staged.Read(payload)
	}

	left := spreadSample.sample(spreadSample.staged, rootKey, channelKeys, sourceKeys[0])
	right := spreadSample.sample(spreadSample.staged, rootKey, channelKeys, sourceKeys[1])
	mid := (left + right) / 2
	spread := 0.0

	if mid > 0 {
		spread = math.Abs(right-left) / mid
	}

	spreadSample.staged.Poke(spread, "sample")

	return spreadSample.staged.Read(payload)
}

func (spreadSample *SpreadSample) sample(
	artifact *datura.Artifact,
	rootKey string,
	channelKeys []string,
	sourceKey string,
) float64 {
	if rootKey == "" || sourceKey == "" || len(channelKeys) == 0 {
		return 0
	}

	for index, channelKey := range channelKeys {
		if channelKey != sourceKey {
			continue
		}

		return datura.Peek[float64](artifact, rootKey, index)
	}

	return 0
}

func (spreadSample *SpreadSample) Close() error {
	return nil
}
