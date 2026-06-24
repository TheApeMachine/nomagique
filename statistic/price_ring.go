package statistic

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
PriceRing retains a bounded sample history and publishes the current observation on the wire.
*/
type PriceRing struct {
	artifact     *datura.Artifact
	priceSamples map[string][]struct {
		value float64
		at    time.Time
	}
}

/*
NewPriceRing returns a sample ring stage wired from config attributes on the artifact.
*/
func NewPriceRing(artifact *datura.Artifact) *PriceRing {
	return &PriceRing{
		artifact: artifact,
		priceSamples: map[string][]struct {
			value float64
			at    time.Time
		}{},
	}
}

func (priceRing *PriceRing) Read(payload []byte) (int, error) {
	state := datura.Acquire("price-ring-state", datura.APPJSON)

	if _, err := state.Write(priceRing.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("statistic", "price-ring", "Read()", "p")

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: inputs required",
			nil,
		))
	}

	stageKey := datura.Peek[string](priceRing.artifact, "stage")

	if stageKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: stage required",
			nil,
		))
	}

	configInput := datura.Peek[string](priceRing.artifact, stageKey, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: input required",
			nil,
		))
	}

	returnLag := int(datura.Peek[float64](priceRing.artifact, stageKey, "returnLag"))

	if returnLag <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: returnLag required",
			nil,
		))
	}

	var sample float64
	sampleFound := false

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"price-ring: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		sampleFound = true
	}

	if !sampleFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: input not in inputs",
			nil,
		))
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: sample is non-positive or non-finite",
			nil,
		))
	}

	longHint := int(datura.Peek[float64](priceRing.artifact, stageKey, "longWindow"))
	seriesKey := stageKey
	scope, _ := state.Scope()

	if scope != "" {
		seriesKey = stageKey + "/" + scope
	}

	timestamp := state.Timestamp()

	if timestamp <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: event timestamp required",
			nil,
		))
	}

	observed := time.Unix(0, timestamp)
	samples := priceRing.priceSamples[seriesKey]

	if len(samples) > 0 && observed.Before(samples[len(samples)-1].at) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: event timestamp must not regress",
			nil,
		))
	}

	samples = append(samples, struct {
		value float64
		at    time.Time
	}{value: sample, at: observed})

	longWindow := len(samples)

	if longHint > 0 {
		longWindow = longHint
	}

	if longWindow > 0 && len(samples) > longWindow+returnLag {
		samples = samples[len(samples)-longWindow-returnLag:]
	}

	priceRing.priceSamples[seriesKey] = samples
	state.MergeOutput(configInput, sample)
	state.Poke("output", "root")
	state.Poke([]string{configInput}, "inputs")

	return state.Read(payload)
}

func (priceRing *PriceRing) Write(payload []byte) (int, error) {
	priceRing.artifact.WithPayload(payload)
	return len(payload), nil
}

func (priceRing *PriceRing) Close() error {
	return nil
}
