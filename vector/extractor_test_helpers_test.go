package vector

/*
newPairExtractor builds a two-input, three-feature extractor for tests.

Features:
  - index 0: sum of channels
  - index 1: difference of channels
  - index 2: product of channels
*/
func newPairExtractor() (*FeatureExtractor, error) {
	return NewFeatureExtractor(2,
		func(inputs []float64) float64 {
			return inputs[0] + inputs[1]
		},
		func(inputs []float64) float64 {
			return inputs[0] - inputs[1]
		},
		func(inputs []float64) float64 {
			return inputs[0] * inputs[1]
		},
	)
}

const (
	testLeftChannel  = 0
	testRightChannel = 1
)

const (
	testSumFeature     = 0
	testDiffFeature    = 1
	testProductFeature = 2
)
