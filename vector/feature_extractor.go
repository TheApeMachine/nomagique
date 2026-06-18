package vector

import (
	"io"

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
	artifact   *datura.Artifact
	transforms map[string]io.ReadWriter
}

/*
NewFeatureExtractor builds an extractor with inputCount channels and one formula
per derived feature, evaluated in registration order.
*/
func NewFeatureExtractor(artifact *datura.Artifact) *FeatureExtractor {
	return &FeatureExtractor{
		artifact:   artifact,
		transforms: make(map[string]io.ReadWriter),
	}
}

func (extractor *FeatureExtractor) Read(p []byte) (int, error) {
	inputs := datura.Peek[[]string](extractor.artifact, "inputs")
	features := datura.Map[[]float64]{
		"features": make([]float64, len(inputs)),
	}

	for index, input := range inputs {
		features["features"][index] = extractor.transform(
			extractor.artifact, input, index,
		)
	}

	payload := errnie.Does(func() ([]byte, error) {
		return sonic.Marshal(features)
	}).Or(func(err error) {
		extractor.artifact.WithError(errnie.Error(errnie.Err(
			errnie.IO,
			"feature-extractor",
			err,
		)))
	}).Value()

	return datura.Acquire(
		"feature-extractor", datura.APPJSON,
	).WithPayload(payload).Read(p)
}

func (extractor *FeatureExtractor) Write(p []byte) (int, error) {
	return extractor.artifact.Write(p)
}

func (extractor *FeatureExtractor) Close() error {
	return nil
}

var transformMap = map[string]func() io.ReadWriter{
	"ema": func() io.ReadWriter { return adaptive.NewEMA() },
}

func (extractor *FeatureExtractor) transform(
	artifact *datura.Artifact,
	input string,
	index int,
) float64 {
	var ok bool

	typedData := datura.PeekPayload[map[string]any](
		artifact, "data", input, index,
	)

	label := typedData["label"].(string)
	value := typedData["value"].(float64)
	transform := typedData["transform"].(string)

	var transformer io.ReadWriter

	if transformer, ok = extractor.transforms[label]; !ok {
		transformer = transformMap[transform]()
		extractor.transforms[label] = transformer
		artifact.Poke("sample", value)
		transport.NewFlipFlop(artifact, transformer)
	}

	return value
}
