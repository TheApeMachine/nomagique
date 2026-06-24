package equation_test

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
)

func writeFeatureStage(stage io.Writer, inputKeys []string, values ...float64) error {
	inbound := datura.Acquire("equation-test-in", datura.APPJSON)
	inbound.WithPayload(equation.MarshalFeatureSchema(inputKeys, values))
	frame := inbound.Pack()

	if len(frame) == 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"writeFeatureStage: inbound marshalling failed",
			nil,
		))
	}

	if _, err := stage.Write(frame); err != nil {
		return errnie.Error(errnie.Err(
			errnie.IO,
			"writeFeatureStage: stage write failed",
			err,
		))
	}

	return nil
}

func readStageOutput(stage io.Reader) (*datura.Artifact, error) {
	chunk := make([]byte, 262144)
	readCount, err := stage.Read(chunk)

	if err != nil && err != io.EOF && err != io.ErrShortBuffer {
		return nil, errnie.Error(errnie.Err(
			errnie.IO,
			"readStageOutput: stage read failed",
			err,
		))
	}

	outbound := datura.Acquire("equation-test-out", datura.Artifact_Type_json)
	_, err = outbound.Write(chunk[:readCount])

	if err != nil {
		outbound.Release()
		return nil, errnie.Error(errnie.Err(
			errnie.IO,
			"readStageOutput: outbound write failed",
			err,
		))
	}

	if !outbound.HasEncryptedPayload() {
		outbound.Release()

		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"readStageOutput: stage produced no output",
			nil,
		))
	}

	return outbound, nil
}

func depthConfig() *datura.Artifact {
	return equation.DepthConfig()
}

func cohortConfig() *datura.Artifact {
	return equation.CohortConfig()
}

func bookflowConfig() *datura.Artifact {
	return equation.BookflowConfig()
}
