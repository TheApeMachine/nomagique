package vector

import (
	"io"
	"math"

	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
)

/*
FeatureExtractor holds raw input channels and derived feature slots in reusable
buffers. Write brings an artifact in; Read updates one channel and refreshes features.
*/
type FeatureExtractor struct {
	config *datura.Artifact
	staged *datura.Artifact
	output *datura.Artifact

	transforms map[string]func(*datura.Artifact) io.ReadWriter
	stages     map[string]io.ReadWriter
}

/*
NewFeatureExtractor builds an extractor wired from schema.inputs payload keys.
*/
func NewFeatureExtractor(artifact *datura.Artifact) *FeatureExtractor {
	return &FeatureExtractor{
		config: artifact,
		staged: datura.Acquire(
			"nomagique", datura.APPJSON,
		).WithRole(
			"feature-extractor",
		).WithScope(
			"staged",
		),
		output: datura.Acquire(
			"nomagique", datura.APPJSON,
		).WithRole(
			"feature-extractor",
		).WithScope(
			"output",
		),
		transforms: map[string]func(*datura.Artifact) io.ReadWriter{
			"ema": func(config *datura.Artifact) io.ReadWriter {
				return adaptive.NewEMA(config)
			},
		},
		stages: make(map[string]io.ReadWriter),
	}
}

func (extractor *FeatureExtractor) Read(p []byte) (int, error) {
	rootKey := datura.Peek[string](extractor.config, "root")
	order := datura.Peek[[]string](extractor.config, "order")
	features := make([]float64, len(order))

	for index, key := range order {
		inputConfig := datura.Peek[map[string]any](extractor.config, "inputs", key)
		features[index] = extractor.sample(extractor.staged, rootKey, key, inputConfig)
	}

	payload := errnie.Does(func() ([]byte, error) {
		return sonic.Marshal(datura.Map[[]float64]{"features": features})
	}).Or(func(err error) {
		extractor.output.WithError(errnie.Error(errnie.Err(
			errnie.IO,
			"feature-extractor",
			err,
		)))
	}).Value()

	extractor.output.WithPayload(payload)

	return extractor.output.Read(p)
}

func (extractor *FeatureExtractor) Write(p []byte) (int, error) {
	return extractor.staged.Write(p)
}

func (extractor *FeatureExtractor) Close() error {
	return nil
}

func (extractor *FeatureExtractor) sample(
	artifact *datura.Artifact,
	rootKey string,
	input string,
	inputConfig map[string]any,
) float64 {
	value := datura.PeekPayload[float64](artifact, rootKey, 0, input)

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	transform, _ := inputConfig["transform"].(string)

	if transform != "ema" {
		return value
	}

	transformer, ok := extractor.stages[input]

	if !ok {
		transformer = extractor.transforms[transform](extractor.config)
		extractor.stages[input] = transformer
	}

	artifact.Poke(value, "sample")
	transport.NewFlipFlop(artifact, transformer)

	return datura.Peek[float64](artifact, "output", "value")
}
