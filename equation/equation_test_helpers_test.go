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
	frame, err := inbound.MarshalPacked()

	if err != nil {
		return err
	}

	_, err = stage.Write(frame)

	return err
}

func readStageOutput(stage io.Reader) (*datura.Artifact, error) {
	chunk := make([]byte, 262144)
	readCount, readErr := stage.Read(chunk)

	if readErr != nil && readErr != io.EOF && readErr != io.ErrShortBuffer {
		return nil, errnie.Error(errnie.Err(errnie.IO, "readStageOutput: stage read failed", readErr))
	}

	outbound := datura.Acquire("equation-test-out", datura.Artifact_Type_json)
	_, writeErr := outbound.Write(chunk[:readCount])

	if writeErr != nil {
		outbound.Release()

		return nil, errnie.Error(errnie.Err(errnie.IO, "readStageOutput: outbound write failed", writeErr))
	}

	if !outbound.HasEncryptedPayload() {
		outbound.Release()

		return nil, errnie.Error(errnie.Err(errnie.Validation, "readStageOutput: stage produced no output", nil))
	}

	return outbound, nil
}
