package algorithm

import (
	"bytes"
	"io"

	"github.com/theapemachine/datura"
)

type fixedScore struct {
	artifact *datura.Artifact
	value    float64
}

func (fixedScore *fixedScore) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (fixedScore *fixedScore) Read(p []byte) (int, error) {
	fixedScore.artifact.WithPayload(encodePayload(fixedScore.value))

	return fixedScore.artifact.Read(p)
}

func (fixedScore *fixedScore) Close() error {
	return nil
}

func readScalar(stage io.ReadWriter, samples ...float64) float64 {
	inbound := datura.Acquire("test-in", datura.Artifact_Type_json)
	inbound.WithPayload(encodePayload(samples...))
	buf, _ := inbound.Message().Marshal()
	_, _ = stage.Write(buf)

	var outBuf bytes.Buffer
	chunk := make([]byte, 4096)

	for {
		readCount, readErr := stage.Read(chunk)

		if readCount > 0 {
			outBuf.Write(chunk[:readCount])
		}

		if readErr == io.EOF {
			break
		}

		if readErr != nil {
			break
		}
	}

	outbound := datura.Acquire("test-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf.Bytes())
	payload, payloadOK := outbound.PayloadQuiet()
	value, ok := payloadScalar(payload)

	if !payloadOK || !ok {
		return 0
	}

	return value
}

func observeInputs(stage io.ReadWriter, series ...float64) float64 {
	return readScalar(stage, series...)
}

func observeWithWork(stage io.ReadWriter, sample float64, work float64) float64 {
	return readScalar(stage, sample, work)
}
