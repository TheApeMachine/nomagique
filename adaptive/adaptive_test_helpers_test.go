package adaptive

import (
	"io"
	"testing"

	"github.com/theapemachine/nomagique/logic"
	"github.com/theapemachine/nomagique/tests"
)

func pipelineSample(stage io.ReadWriter, sample float64) float64 {
	value, _ := tests.PipelineSample([]io.ReadWriter{stage}, sample)

	return value
}

func pipelineSamples(stage io.ReadWriter, samples ...float64) float64 {
	return tests.RunObserveSampleSequence(func(sample float64) float64 {
		return pipelineSample(stage, sample)
	}, samples)
}

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

func observeWithWork(stage io.ReadWriter, sample float64, work float64) float64 {
	if writeErr := tests.WriteSamples(stage, sample, work); writeErr != nil {
		return 0
	}

	value, _ := tests.ReadSample(stage)

	return value
}

func observeWithCombinedWork(stage io.ReadWriter, sample float64, work float64) float64 {
	value, _ := tests.PipelineSample([]io.ReadWriter{stage}, sample+work)

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

func mustConstants(testingTB testing.TB, series ...float64) []io.ReadWriter {
	testingTB.Helper()

	stages := make([]io.ReadWriter, len(series))

	for index, sample := range series {
		stages[index] = logic.NewConstant(sample)
	}

	return stages
}
