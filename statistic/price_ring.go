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
	artifact     *datura.Artifact
	priceSamples map[string][]float64
}

/*
NewPriceRing returns a sample ring stage wired from config attributes on the artifact.
*/
func NewPriceRing(artifact *datura.Artifact) *PriceRing {
	artifact.Inspect("statistic", "price-ring", "NewPriceRing()")

	return &PriceRing{
		artifact:     artifact,
		priceSamples: map[string][]float64{},
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

	stageKey := priceRing.stageKey()

	if stageKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: stage config required",
			nil,
		))
	}

	sourceKey := datura.Peek[string](priceRing.artifact, stageKey, "input")

	if sourceKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: input key required",
			nil,
		))
	}

	sample, sampleErr := FeatureColumn(state, sourceKey)

	if sampleErr != nil {
		root := datura.Peek[string](state, "root")
		sample = datura.Peek[float64](state, root, sourceKey)
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: sample is non-positive or non-finite",
			nil,
		))
	}

	longHint := int(datura.Peek[float64](priceRing.artifact, stageKey, "longWindow"))

	returnLag, err := ReturnLag(
		priceRing.priceSamples[stageKey],
		int(datura.Peek[float64](priceRing.artifact, stageKey, "returnLag")),
		longHint,
	)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: unable to resolve return lag",
			err,
		))
	}

	samples := priceRing.priceSamples[stageKey]
	samples = append(samples, sample)

	_, longWindow, err := RollingWindows(samples, 0, longHint)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: unable to resolve long window",
			err,
		))
	}

	if longWindow > 0 && len(samples) > longWindow+returnLag {
		samples = samples[len(samples)-longWindow-returnLag:]
	}

	priceRing.priceSamples[stageKey] = samples
	state.MergeOutput(sourceKey, sample)
	state.Poke("output", "root")
	state.Poke([]string{sourceKey}, "inputs")

	return state.Read(payload)
}

func (priceRing *PriceRing) stageKey() string {
	stageKey := datura.Peek[string](priceRing.artifact, "stage")

	if stageKey != "" {
		return stageKey
	}

	order := datura.Peek[[]string](priceRing.artifact, "order")
	stageIndex := int(datura.Peek[float64](priceRing.artifact, "precursor", "stageIndex"))

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
