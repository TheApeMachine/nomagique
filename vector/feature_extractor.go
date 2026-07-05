package vector

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
)

/*
FeatureScopeConfig describes how one channel maps typed rows to features.
*/
type FeatureScopeConfig struct {
	Root         string
	Inputs       []string
	ElementIndex int
	Transforms   map[string]string
}

/*
FeatureExtractorConfig describes global and channel-specific feature scopes.
*/
type FeatureExtractorConfig struct {
	FeatureScopeConfig
	Channels map[string]FeatureScopeConfig
}

/*
FeatureExtractor extracts typed feature vectors from market frames.
*/
type FeatureExtractor struct {
	config FeatureExtractorConfig
	emas   map[string]*adaptive.EMA
}

/*
NewFeatureExtractor builds an extractor from a typed schema.
*/
func NewFeatureExtractor(config FeatureExtractorConfig) *FeatureExtractor {
	return &FeatureExtractor{
		config: config,
		emas:   map[string]*adaptive.EMA{},
	}
}

/*
Measure extracts the configured feature vector from the typed frame.
*/
func (extractor *FeatureExtractor) Measure(input FeatureInput) (FeatureVector, error) {
	scope := extractor.scope(input.Channel)

	if scope.Root == "" {
		return FeatureVector{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: root required",
			nil,
		))
	}

	if len(scope.Inputs) == 0 {
		return FeatureVector{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: inputs required",
			nil,
		))
	}

	row, err := extractor.row(input, scope)
	if err != nil {
		return FeatureVector{}, err
	}

	features := make([]float64, len(scope.Inputs))

	for index, inputKey := range scope.Inputs {
		sample, exists := row.Value(inputKey)

		if !exists {
			return FeatureVector{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"feature-extractor: input not found: "+inputKey,
				nil,
			))
		}

		if err := finiteVector("feature-extractor: "+inputKey, sample); err != nil {
			return FeatureVector{}, err
		}

		transform := scope.Transforms[inputKey]

		if transform == "" {
			features[index] = sample
			continue
		}

		transformKey := inputKey

		if input.Channel != "" {
			transformKey = input.Channel + ":" + inputKey
		}

		sample, err = extractor.transform(transform, transformKey, sample)
		if err != nil {
			return FeatureVector{}, err
		}

		features[index] = sample
	}

	return FeatureVector{
		Features:     features,
		Inputs:       append([]string(nil), scope.Inputs...),
		SourceRoot:   scope.Root,
		SourceInputs: append([]string(nil), scope.Inputs...),
	}, nil
}

func (extractor *FeatureExtractor) scope(channel string) FeatureScopeConfig {
	scope := extractor.config.FeatureScopeConfig

	if channel == "" {
		return scope
	}

	scoped, exists := extractor.config.Channels[channel]
	if !exists {
		return scope
	}

	if scoped.Root != "" {
		scope.Root = scoped.Root
	}

	if len(scoped.Inputs) > 0 {
		scope.Inputs = scoped.Inputs
	}

	if scoped.ElementIndex != 0 {
		scope.ElementIndex = scoped.ElementIndex
	}

	if len(scoped.Transforms) > 0 {
		scope.Transforms = scoped.Transforms
	}

	return scope
}

func (extractor *FeatureExtractor) row(
	input FeatureInput,
	scope FeatureScopeConfig,
) (FeatureRow, error) {
	if scope.Root == "." {
		return input.Row, nil
	}

	if scope.ElementIndex < 0 || scope.ElementIndex >= len(input.Rows) {
		return FeatureRow{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: element index out of range",
			nil,
		))
	}

	return input.Rows[scope.ElementIndex], nil
}

func (extractor *FeatureExtractor) transform(
	transform string,
	key string,
	sample float64,
) (float64, error) {
	if transform != "ema" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: transform not registered",
			nil,
		))
	}

	ema, exists := extractor.emas[key]
	if !exists {
		ema = adaptive.NewEMA()
		extractor.emas[key] = ema
	}

	out, err := ema.Measure(sample)
	if err != nil {
		return 0, err
	}

	if err := finiteVector("feature-extractor: transform", out); err != nil {
		return 0, err
	}

	return out, nil
}
