package algorithm

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

func collectScalars[T ~float64](inputs ...core.Number[T]) ([]float64, bool) {
	scalars := make([]float64, 0, len(inputs))

	for _, input := range inputs {
		scalar, ok := input.(core.Scalar[T])

		if !ok {
			return nil, false
		}

		scalars = append(scalars, float64(scalar))
	}

	return scalars, true
}

func samplesToInputs[T ~float64](samples []float64) []core.Number[T] {
	inputs := make([]core.Number[T], len(samples))

	for index, sample := range samples {
		inputs[index] = core.Scalar[T](T(sample))
	}

	return inputs
}

func parsePredictedActual(
	primary float64, extras []float64,
) (float64, float64, error) {
	if len(extras) >= 2 {
		predicted := extras[0]
		actual := extras[1]

		if predicted == 0 {
			return 0, 0, core.ErrZeroPredicted
		}

		return predicted, actual, nil
	}

	if len(extras) == 0 {
		return 0, 0, core.ErrEmptyInputs
	}

	predicted := primary
	actual := extras[0]

	if predicted == 0 {
		return 0, 0, core.ErrZeroPredicted
	}

	return predicted, actual, nil
}

func zipNodeRows(streams [][]float64) ([][]float64, bool) {
	if len(streams) == 0 {
		return nil, false
	}

	rowCount := len(streams[0])

	if rowCount == 0 {
		return nil, false
	}

	rows := make([][]float64, rowCount)

	for rowIndex := range rows {
		rows[rowIndex] = make([]float64, len(streams))

		for nodeIndex, stream := range streams {
			if len(stream) != rowCount {
				return nil, false
			}

			rows[rowIndex][nodeIndex] = stream[rowIndex]
		}
	}

	return rows, true
}
