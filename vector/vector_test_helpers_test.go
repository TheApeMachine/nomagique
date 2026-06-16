package vector

import (
	"io"

	"github.com/theapemachine/nomagique/tests"
)

func observeInputs(stage io.ReadWriter, series ...float64) float64 {
	if len(series) == 0 {
		value, _ := tests.PipelineSample([]io.ReadWriter{stage}, 0)

		return value
	}

	if len(series) == 1 {
		value, _ := tests.PipelineSample([]io.ReadWriter{stage}, series[0])

		return value
	}

	if writeErr := tests.WriteSamples(stage, series...); writeErr != nil {
		return 0
	}

	value, _ := tests.ReadSample(stage)

	return value
}

func observeWithoutSample(stage io.ReadWriter, carried float64) float64 {
	_ = carried

	if valueStage, ok := stage.(interface{ Value() float64 }); ok {
		return valueStage.Value()
	}

	value, _ := tests.ReadSample(stage)

	return value
}

func observeWithWork(stage io.ReadWriter, sample float64, work float64) float64 {
	if writeErr := tests.WriteSamples(stage, sample, work); writeErr != nil {
		return 0
	}

	value, _ := tests.ReadSample(stage)

	return value
}
