package logic

import (
	"io"
	"testing"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/tests"
)

func matchCondition(condition Condition, carried float64) bool {
	return condition.Match(tests.SampleArtifact(carried))
}

func matchConditionWithoutSample(condition Condition) bool {
	return condition.Match(tests.SampleArtifact(0))
}

func mustConstants(testingTB testing.TB, series ...float64) []io.ReadWriter {
	testingTB.Helper()

	stages := make([]io.ReadWriter, len(series))

	for index, sample := range series {
		stages[index] = NewConstant(sample)
	}

	return stages
}

func pipelineCircuit(circuit *Circuit, sample float64) float64 {
	value, _ := tests.PipelineSample([]io.ReadWriter{circuit}, sample)

	return value
}

func matchArtifact(sample float64) *datura.Artifact {
	return tests.SampleArtifact(sample)
}
