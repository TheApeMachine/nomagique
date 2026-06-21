package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
PriceRing retains a bounded sample history and publishes the current observation on the wire.
The constructor artifact holds config; runtime history lives on the stage instance.
*/
type PriceRing struct {
	artifact *datura.Artifact
	samples  []float64
}

/*
NewPriceRing returns a sample ring stage wired from config attributes on the artifact.
*/
func NewPriceRing(artifact *datura.Artifact) *PriceRing {
	artifact.Inspect("statistic", "price-ring", "NewPriceRing()")

	return &PriceRing{
		artifact: artifact,
		samples:  []float64{},
	}
}

func (priceRing *PriceRing) Write(payload []byte) (int, error) {
	priceRing.artifact.WithPayload(payload)

	return len(payload), nil
}

func (priceRing *PriceRing) Read(payload []byte) (int, error) {
	state := datura.Acquire("price-ring-state", datura.APPJSON)
	state.Inspect("statistic", "price-ring", "Read()", "p")

	if _, err := state.Write(priceRing.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := SnapshotFeatures(state)
	stageKey := priceRing.stageKey()

	if stageKey == "" {
		features.Restore(state)

		return state.Read(payload)
	}

	sourceKey := datura.Peek[string](priceRing.artifact, "inputs", stageKey, "input")
	sample := FeatureColumn(state, sourceKey)

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: sample is NaN or Inf",
			nil,
		))
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		features.Restore(state)

		return state.Read(payload)
	}

	returnLag := int(datura.Peek[float64](priceRing.artifact, "inputs", stageKey, "returnLag"))
	longHint := int(datura.Peek[float64](priceRing.artifact, "inputs", stageKey, "longWindow"))

	if returnLag <= 0 {
		returnLag = 1
	}

	samples := priceRing.samples
	samples = append(samples, sample)

	_, longWindow := RollingWindows(samples, 0, longHint)

	if longWindow > 0 && len(samples) > longWindow+returnLag {
		samples = samples[len(samples)-longWindow-returnLag:]
	}

	priceRing.samples = samples
	features.Restore(state)
	state.Merge("sample", sample)
	state.Merge("root", "sample")

	return state.Read(payload)
}

func (priceRing *PriceRing) stageKey() string {
	stageKey := datura.Peek[string](priceRing.artifact, "stage")

	if stageKey != "" {
		return stageKey
	}

	order := datura.Peek[[]string](priceRing.artifact, "order")
	stageIndex := int(datura.Peek[float64](priceRing.artifact, "inputs", "precursor", "stageIndex"))

	if stageIndex <= 0 {
		stageIndex = int(datura.Peek[float64](priceRing.artifact, "stageIndex"))
	}

	if stageIndex >= 0 && len(order) > stageIndex {
		return order[stageIndex]
	}

	return ""
}

func (priceRing *PriceRing) Close() error {
	return nil
}
