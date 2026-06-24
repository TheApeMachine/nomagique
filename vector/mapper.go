package vector

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Mapper derives new named values from existing wire values. Each mapping reads one
or more named inputs, optionally combines them (sum/product/mean/ratio/diff/min/max),
optionally wraps the result in a transform (e.g. ema), and writes it under outputKey.
Driven entirely by the "mappings" config attribute, generic across signals.
*/
type Mapper struct {
	artifact     *datura.Artifact
	transforms   map[string]func(*datura.Artifact) io.ReadWriteCloser
	transformers map[string]io.ReadWriteCloser
}

/*
NewMapper builds a mapper wired from the "mappings" config attribute.
*/
func NewMapper(artifact *datura.Artifact) *Mapper {
	return &Mapper{
		artifact: artifact,
		transforms: map[string]func(*datura.Artifact) io.ReadWriteCloser{
			"ema": func(config *datura.Artifact) io.ReadWriteCloser {
				return adaptive.NewEMA(config)
			},
		},
		transformers: map[string]io.ReadWriteCloser{},
	}
}

func (mapper *Mapper) Read(payload []byte) (int, error) {
	state := datura.Acquire("mapper-state", datura.APPJSON)
	defer state.Release()

	if _, err := state.Write(mapper.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "mapper: state write failed", err,
		))
	}

	snapshot := statistic.SnapshotFeatures(state)
	mappings := datura.Peek[[]any](mapper.artifact, "mappings")

	if len(mappings) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "mapper: mappings required", nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	for index := range mappings {
		outputKey := datura.Peek[string](mapper.artifact, "mappings", index, "outputKey")

		if outputKey == "" {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "mapper: mapping outputKey required", nil,
			))
		}

		value, err := mapper.value(state, index)

		if err != nil {
			return 0, err
		}

		state.MergeOutput(outputKey, value)
		inputs = appendInput(inputs, outputKey)
	}

	snapshot.Restore(state)
	state.Poke("output", "root")
	state.Poke(inputs, "inputs")

	return state.Read(payload)
}

/*
value computes one mapping: combine its named inputs, then apply any transform.
*/
func (mapper *Mapper) value(state *datura.Artifact, index int) (float64, error) {
	keys := datura.Peek[[]string](mapper.artifact, "mappings", index, "inputs")

	if len(keys) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "mapper: mapping inputs required", nil,
		))
	}

	inverts := datura.Peek[[]string](mapper.artifact, "mappings", index, "inverts")
	op := datura.Peek[string](mapper.artifact, "mappings", index, "op")
	samples := make([]float64, 0, len(keys))

	for _, key := range keys {
		sample, err := readValue(state, key)

		if err != nil {
			return 0, err
		}

		samples = append(samples, invert(sample, contains(inverts, key), op))
	}

	combined, err := combine(op, samples)

	if err != nil {
		return 0, err
	}

	transform := datura.Peek[string](mapper.artifact, "mappings", index, "transform")

	if transform == "" {
		return combined, nil
	}

	outputKey := datura.Peek[string](mapper.artifact, "mappings", index, "outputKey")

	return mapper.apply(transform, outputKey, combined)
}

/*
apply wraps a scalar in a stateful transform stage, reusing one persistent
instance per outputKey so transforms like ema accumulate across frames.
*/
func (mapper *Mapper) apply(transform, outputKey string, value float64) (float64, error) {
	factory, ok := mapper.transforms[transform]

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "mapper: transform not registered", nil,
		))
	}

	stage, ok := mapper.transformers[outputKey]

	if !ok {
		config := datura.Acquire("mapper-transform-"+outputKey, datura.APPJSON)
		config.Poke("sample", "input")
		stage = factory(config)
		mapper.transformers[outputKey] = stage
	}

	scratch := datura.Acquire("mapper-transform-scratch", datura.APPJSON)
	defer scratch.Release()
	scratch.WithPayload(datura.Map[any]{
		"features": []float64{value},
		"root":     "features",
		"inputs":   []string{"sample"},
	}.Marshal())

	if err := transport.NewFlipFlop(scratch, stage); err != nil {
		return 0, err
	}

	scratchRoot := datura.Peek[string](scratch, "root")
	out := datura.Peek[float64](scratch, scratchRoot, "value")

	if math.IsNaN(out) || math.IsInf(out, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "mapper: transform output non-finite", nil,
		))
	}

	return out, nil
}

func (mapper *Mapper) Write(payload []byte) (int, error) {
	mapper.artifact.WithPayload(payload)
	return len(payload), nil
}

func (mapper *Mapper) Close() error {
	return nil
}

/*
readValue resolves a named value from the wire in either mode: positionally from
the features slice when root is "features", or by key from the output map.
*/
func readValue(state *datura.Artifact, name string) (float64, error) {
	rootKey := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if rootKey == "features" {
		features := datura.Peek[[]float64](state, "features")

		for index, key := range inputs {
			if key != name || index >= len(features) {
				continue
			}

			return finite(name, features[index])
		}

		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "mapper: input not found in features: "+name, nil,
		))
	}

	return finite(name, datura.Peek[float64](state, rootKey, name))
}

func combine(op string, samples []float64) (float64, error) {
	if len(samples) == 1 && (op == "" || op == "raw") {
		return samples[0], nil
	}

	switch op {
	case "sum", "":
		total := 0.0
		for _, sample := range samples {
			total += sample
		}
		return total, nil
	case "mean":
		total := 0.0
		for _, sample := range samples {
			total += sample
		}
		return total / float64(len(samples)), nil
	case "product":
		total := 1.0
		for _, sample := range samples {
			total *= sample
		}
		return total, nil
	case "diff":
		return fold(samples, func(acc, sample float64) float64 { return acc - sample }), nil
	case "ratio":
		return ratio(samples)
	case "min":
		return fold(samples, math.Min), nil
	case "max":
		return fold(samples, math.Max), nil
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation, "mapper: unknown op: "+op, nil,
	))
}

func ratio(samples []float64) (float64, error) {
	acc := samples[0]

	for _, sample := range samples[1:] {
		if sample == 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "mapper: ratio divide by zero", nil,
			))
		}

		acc /= sample
	}

	return acc, nil
}

func fold(samples []float64, step func(float64, float64) float64) float64 {
	acc := samples[0]

	for _, sample := range samples[1:] {
		acc = step(acc, sample)
	}

	return acc
}

/*
invert flips a value's contribution: additive negation for additive ops,
reciprocal for multiplicative ops.
*/
func invert(value float64, doInvert bool, op string) float64 {
	if !doInvert {
		return value
	}

	if op == "product" || op == "ratio" {
		if value == 0 {
			return 0
		}

		return 1 / value
	}

	return -value
}

func finite(name string, value float64) (float64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "mapper: non-finite value: "+name, nil,
		))
	}

	return value, nil
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}

	return false
}

func appendInput(inputs []string, key string) []string {
	if contains(inputs, key) {
		return inputs
	}

	return append(append([]string(nil), inputs...), key)
}
