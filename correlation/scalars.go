package correlation

import (
	"github.com/theapemachine/nomagique/core"
)

func sampleBatch[T ~float64](inputs ...core.Number[T]) []float64 {
	values := make([]float64, 0, len(inputs))

	for _, input := range inputs {
		scalar, ok := input.(core.Scalar[T])

		if !ok {
			continue
		}

		values = append(values, float64(scalar))
	}

	return values
}

func parseEpochLevel[T ~float64](
	inputs ...core.Number[T],
) (epoch int64, level float64, ok bool) {
	if len(inputs) < 2 {
		return 0, 0, false
	}

	epochScalar, epochOK := inputs[0].(core.Scalar[T])
	levelScalar, levelOK := inputs[1].(core.Scalar[T])

	if !epochOK || !levelOK {
		return 0, 0, false
	}

	return int64(float64(epochScalar)), float64(levelScalar), true
}

func weightSamples(weights []float64) []float64 {
	if len(weights) == 0 {
		return nil
	}

	return weights
}
