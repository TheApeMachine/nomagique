package vector

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
)

/*
Mapping describes one derived named scalar.
*/
type Mapping struct {
	OutputKey string
	Inputs    []string
	Inverts   []string
	Op        string
	Transform string
}

/*
MapperConfig describes all derived scalar mappings.
*/
type MapperConfig struct {
	Mappings []Mapping
}

/*
Mapper derives new named values from existing vector values.
*/
type Mapper struct {
	config MapperConfig
	emas   map[string]*adaptive.EMA
}

/*
NewMapper builds a mapper from typed mapping config.
*/
func NewMapper(config MapperConfig) *Mapper {
	return &Mapper{
		config: config,
		emas:   map[string]*adaptive.EMA{},
	}
}

/*
Measure applies all configured mappings to the feature vector.
*/
func (mapper *Mapper) Measure(vector FeatureVector) (FeatureVector, error) {
	if len(mapper.config.Mappings) == 0 {
		return FeatureVector{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"mapper: mappings required",
			nil,
		))
	}

	for _, mapping := range mapper.config.Mappings {
		if mapping.OutputKey == "" {
			return FeatureVector{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"mapper: mapping outputKey required",
				nil,
			))
		}

		value, err := mapper.value(vector, mapping)
		if err != nil {
			return FeatureVector{}, err
		}

		vector = vector.WithValue(NamedValue{
			Name:  mapping.OutputKey,
			Value: value,
		})
	}

	return vector, nil
}

func (mapper *Mapper) value(
	vector FeatureVector,
	mapping Mapping,
) (float64, error) {
	if len(mapping.Inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mapper: mapping inputs required",
			nil,
		))
	}

	samples := make([]float64, 0, len(mapping.Inputs))

	for _, key := range mapping.Inputs {
		sample, exists := vector.Value(key)

		if !exists {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"mapper: input not found: "+key,
				nil,
			))
		}

		if err := finiteVector("mapper: "+key, sample); err != nil {
			return 0, err
		}

		samples = append(
			samples,
			invert(sample, contains(mapping.Inverts, key), mapping.Op),
		)
	}

	combined, err := combine(mapping.Op, samples)
	if err != nil {
		return 0, err
	}

	if mapping.Transform == "" {
		return combined, nil
	}

	return mapper.apply(mapping.Transform, mapping.OutputKey, combined)
}

func (mapper *Mapper) apply(
	transform string,
	outputKey string,
	value float64,
) (float64, error) {
	if transform != "ema" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"mapper: transform not registered",
			nil,
		))
	}

	ema, exists := mapper.emas[outputKey]
	if !exists {
		ema = adaptive.NewEMA()
		mapper.emas[outputKey] = ema
	}

	out, err := ema.Measure(value)
	if err != nil {
		return 0, err
	}

	if err := finiteVector("mapper: transform", out); err != nil {
		return 0, err
	}

	return out, nil
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
		errnie.Validation,
		"mapper: unknown op: "+op,
		nil,
	))
}

func ratio(samples []float64) (float64, error) {
	accumulator := samples[0]

	for _, sample := range samples[1:] {
		if sample == 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"mapper: ratio divide by zero",
				nil,
			))
		}

		accumulator /= sample
	}

	return accumulator, nil
}

func fold(samples []float64, step func(float64, float64) float64) float64 {
	accumulator := samples[0]

	for _, sample := range samples[1:] {
		accumulator = step(accumulator, sample)
	}

	return accumulator
}

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

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}

	return false
}
