package vector

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
)

/*
FeatureExtractor is an atomic compute primitive (io.ReadWriteCloser).

The constructor artifact is config (attributes). Write buffers inbound wire.
Read unpacks the frame, extracts features, and emits a packed artifact.
*/
type FeatureExtractor struct {
	artifact   *datura.Artifact
	bytes      []byte
	emaConfigs map[string]*datura.Artifact
	transforms map[string]func(*datura.Artifact) io.ReadWriteCloser
}

/*
NewFeatureExtractor builds an extractor wired from schema attributes.
*/
func NewFeatureExtractor(artifact *datura.Artifact) *FeatureExtractor {
	return &FeatureExtractor{
		artifact:   artifact,
		emaConfigs: map[string]*datura.Artifact{},
		transforms: map[string]func(*datura.Artifact) io.ReadWriteCloser{
			"ema": func(config *datura.Artifact) io.ReadWriteCloser {
				return adaptive.NewEMA(config)
			},
		},
	}
}

func (extractor *FeatureExtractor) Read(payload []byte) (int, error) {
	state := datura.Acquire("feature-extractor-state", datura.APPJSON)

	if _, err := state.Write(extractor.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	role := datura.Peek[string](state, "channel")

	rootKey := datura.Peek[string](extractor.artifact, role, "root")
	inputs := datura.Peek[[]string](extractor.artifact, role, "inputs")

	if rootKey == "" {
		rootKey = datura.Peek[string](extractor.artifact, "root")
	}

	if len(inputs) == 0 {
		inputs = datura.Peek[[]string](extractor.artifact, "inputs")
	}

	features := make([]float64, len(inputs))

	for index, input := range inputs {
		sample := datura.Peek[float64](state, rootKey, 0, input)

		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			errnie.Error(errnie.Err(
				errnie.Validation,
				"feature-extractor: sample is NaN or Inf",
				nil,
			))
		}

		transform := datura.Peek[string](extractor.artifact, role, "transforms", input)

		if transform == "" {
			transform = datura.Peek[string](extractor.artifact, "transforms", input)
		}

		if transformer, ok := extractor.transforms[transform]; ok {
			scratch := datura.Acquire("feature-extractor-scratch", datura.APPJSON)
			scratch.WithPayload(datura.Map[any]{"sample": sample}.Marshal())

			config, exists := extractor.emaConfigs[input]

			if !exists {
				config = datura.Acquire("feature-extractor-ema", datura.APPJSON)
				extractor.emaConfigs[input] = config
			}

			transport.NewFlipFlop(scratch, transformer(config))

			sample = datura.Peek[float64](scratch, "output", "value")
			scratch.Release()
		}

		features[index] = sample
	}

	output := datura.Acquire("feature-extractor-output", datura.APPJSON)

	output.WithPayload(datura.Map[[]float64]{
		"features": features,
	}.Marshal())

	output.Merge("root", "features")
	output.Merge("inputs", inputs)

	return output.Read(payload)
}

func (extractor *FeatureExtractor) Write(p []byte) (int, error) {
	extractor.bytes = append(extractor.bytes[:0], p...)

	return len(p), nil
}

func (extractor *FeatureExtractor) Close() error {
	return nil
}
