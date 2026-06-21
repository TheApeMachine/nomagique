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

The constructor artifact is config (attributes); its payload buses inbound wire
from Write to Read. Read unpacks the frame, extracts features, and emits output.
*/
type FeatureExtractor struct {
	artifact   *datura.Artifact
	transforms map[string]func(*datura.Artifact) io.ReadWriteCloser
}

/*
NewFeatureExtractor builds an extractor wired from schema attributes.
*/
func NewFeatureExtractor(artifact *datura.Artifact) *FeatureExtractor {
	artifact.Inspect("feature-extractor", "NewFeatureExtractor()")

	return &FeatureExtractor{
		artifact: artifact,
		transforms: map[string]func(*datura.Artifact) io.ReadWriteCloser{
			"ema": func(config *datura.Artifact) io.ReadWriteCloser {
				return adaptive.NewEMA(config)
			},
		},
	}
}

func (extractor *FeatureExtractor) Read(payload []byte) (int, error) {
	state := datura.Acquire("feature-extractor-state", datura.APPJSON)
	state.Inspect("feature-extractor", "Read()", "p")

	if _, err := state.Write(extractor.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	role := datura.Peek[string](state, "channel")

	if role == "" {
		role, _ = state.Role()
	}

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

			config := datura.Acquire("feature-extractor-ema", datura.APPJSON)
			scratch.Inspect("feature-extractor", "Read()", "transform", transform, "in")
			transport.NewFlipFlop(scratch, transformer(config))
			scratch.Inspect("feature-extractor", "Read()", "transform", transform, "out")

			rootKey := datura.Peek[string](scratch, "root")

			if rootKey == "" {
				rootKey = "output"
			}

			sample = datura.Peek[float64](scratch, rootKey, "value")
		}

		features[index] = sample
	}

	state.Merge("features", features)
	state.Merge("root", "features")
	state.Merge("inputs", inputs)

	return state.Read(payload)
}

func (extractor *FeatureExtractor) Write(p []byte) (int, error) {
	extractor.artifact.WithPayload(p)
	return len(p), nil
}

func (extractor *FeatureExtractor) Close() error {
	return nil
}
