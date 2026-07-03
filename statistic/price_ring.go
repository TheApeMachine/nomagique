package statistic

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
PriceRing publishes the configured input sample on the outbound wire.
*/
type PriceRing struct {
	artifact     *datura.Artifact
	pendingFrame bool
	output       []byte
}

/*
NewPriceRing returns a sample ring stage wired from config attributes on the artifact.
*/
func NewPriceRing(artifact *datura.Artifact) *PriceRing {
	return &PriceRing{
		artifact: artifact,
	}
}

func (priceRing *PriceRing) Read(payload []byte) (int, error) {
	if len(priceRing.output) > 0 {
		return priceRing.readOutput(payload)
	}

	if !priceRing.pendingFrame {
		return 0, io.EOF
	}

	state := datura.Acquire("price-ring-state", datura.APPJSON)
	frame := priceRing.artifact.DecryptPayload()

	if len(frame) == 0 {
		state.Release()
		priceRing.pendingFrame = false

		return 0, io.EOF
	}

	if _, err := state.Unpack(frame); err != nil {
		state.Release()
		priceRing.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: state write failed",
			err,
		))
	}

	defer state.Release()

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		priceRing.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		priceRing.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: inputs required",
			nil,
		))
	}

	stageKey := datura.Peek[string](priceRing.artifact, "block")

	if stageKey == "" {
		stageKey = datura.Peek[string](priceRing.artifact, "stage")
	}

	stageKey = stageConfigKey(priceRing.artifact, stageKey)
	configInput := configString(priceRing.artifact, stageKey, "input")

	if configInput == "" {
		priceRing.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: input required",
			nil,
		))
	}

	var sample float64
	sampleFound := false

	featureSlice := datura.Peek[[]float64](state, "features")
	featureInputs := datura.Peek[[]string](state, "featureInputs")

	if len(featureInputs) == 0 {
		featureInputs = inputs
	}

	for index, input := range featureInputs {
		if input != configInput || index >= len(featureSlice) {
			continue
		}

		sample = featureSlice[index]
		sampleFound = true
	}

	for index, input := range inputs {
		if sampleFound {
			break
		}

		if input != configInput {
			continue
		}

		if rootKey == "features" {
			featureSlice := datura.Peek[[]float64](state, rootKey)

			if index >= len(featureSlice) {
				priceRing.pendingFrame = false

				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"price-ring: feature index out of range",
					nil,
				))
			}

			sample = featureSlice[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		sampleFound = true
	}

	if !sampleFound {
		priceRing.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: input not in inputs",
			nil,
		))
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		priceRing.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"price-ring: sample is non-positive or non-finite",
			nil,
		))
	}

	state.MergeOutput(configInput, sample)
	state.Poke("output", "root")
	state.Poke([]string{configInput}, "inputs")

	priceRing.output = state.Pack()

	return priceRing.readOutput(payload)
}

func (priceRing *PriceRing) Write(payload []byte) (int, error) {
	if len(payload) == 0 {
		priceRing.pendingFrame = false
		priceRing.output = nil

		return 0, nil
	}

	priceRing.artifact.WithPayload(payload)
	priceRing.pendingFrame = true
	priceRing.output = nil

	return len(payload), nil
}

func (priceRing *PriceRing) readOutput(payload []byte) (int, error) {
	n := copy(payload, priceRing.output)

	if n < len(priceRing.output) {
		return n, io.ErrShortBuffer
	}

	priceRing.output = nil
	priceRing.pendingFrame = false

	return n, io.EOF
}

func (priceRing *PriceRing) Close() error {
	return nil
}
