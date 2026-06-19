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
FeatureExtractor holds raw input channels and derived feature slots in reusable
buffers. Write brings an artifact in; Read updates one channel and refreshes features.
*/
type FeatureExtractor struct {
	artifact   *datura.Artifact
	transforms map[string]func(*datura.Artifact) io.ReadWriteCloser
}

/*
NewFeatureExtractor builds an extractor wired from schema.inputs payload keys.
*/
func NewFeatureExtractor(artifact *datura.Artifact) *FeatureExtractor {
	return &FeatureExtractor{
		artifact: artifact,
		transforms: map[string]func(*datura.Artifact) io.ReadWriteCloser{
			"ema": func(config *datura.Artifact) io.ReadWriteCloser {
				return adaptive.NewEMA(config)
			},
		},
	}
}

func (extractor *FeatureExtractor) Read(p []byte) (int, error) {
	rootKey := datura.Peek[string](extractor.artifact, "root")
	inputs := datura.Peek[[]string](extractor.artifact, "inputs")

	features := make([]float64, len(inputs))

	for index, input := range inputs {
		features[index] = extractor.sample(extractor.artifact, rootKey, input)
	}

	extractor.artifact.WithPayload(
		datura.Map[[]float64]{"features": features}.Marshal(),
	).Poke(
		"features", "root",
	).Poke(
		inputs, "inputs",
	)

	return extractor.artifact.Read(p)
}

func (extractor *FeatureExtractor) Write(p []byte) (int, error) {
	return extractor.artifact.Write(p)
}

func (extractor *FeatureExtractor) Close() error {
	return nil
}

func (extractor *FeatureExtractor) sample(
	artifact *datura.Artifact,
	rootKey string,
	input string,
) float64 {
	sample := datura.Peek[float64](artifact, rootKey, input)

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"feature-extractor: sample is NaN or Inf",
			nil,
		))

		return 0
	}

	var (
		transform   = datura.Peek[string](artifact, "transforms", input)
		transformer func(*datura.Artifact) io.ReadWriteCloser
		ok          bool
	)

	if transformer, ok = extractor.transforms[transform]; !ok {
		return sample
	}

	results := datura.Acquire(
		"feature-extractor", datura.APPJSON,
	).WithRole(
		"sample",
	).WithScope(
		input,
	).WithPayload(
		datura.Map[float64]{"sample": sample}.Marshal(),
	).Poke(
		"sample", "rootKey",
	).Poke(
		[]string{input}, "inputs",
	)

	errnie.Error(
		transport.NewFlipFlop(results, transformer(artifact)),
	)

	return datura.Peek[float64](results, "output", 0)
}
