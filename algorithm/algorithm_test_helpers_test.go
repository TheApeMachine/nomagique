package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

type fixedScore struct {
	artifact *datura.Artifact
	value    float64
}

func (fixedScore *fixedScore) Write(p []byte) (int, error) {
	fixedScore.artifact.WithPayload(p)
	return len(p), nil
}

func (fixedScore *fixedScore) Read(p []byte) (int, error) {
	state := datura.Acquire("fixed-score-state", datura.APPJSON)

	if _, err := state.Write(fixedScore.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.MergeOutput("value", fixedScore.value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(p)
}

func (fixedScore *fixedScore) Close() error {
	return nil
}

func readOutbound(stage io.Reader) *datura.Artifact {
	chunk := make([]byte, 262144)
	readCount, readErr := stage.Read(chunk)

	if readErr != nil && readErr != io.EOF && readErr != io.ErrShortBuffer {
		outbound := datura.Acquire("test-out", datura.Artifact_Type_json)

		return outbound
	}

	outbound := datura.Acquire("test-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(chunk[:readCount])

	return outbound
}

func readScalar(stage io.ReadWriter, samples ...float64) float64 {
	inbound := datura.Acquire("test-in", datura.Artifact_Type_json)
	inbound.WithPayload(equation.MarshalFeaturesPayload(samples))
	buf, err := inbound.MarshalPacked()

	if err != nil {
		return 0
	}

	_, _ = stage.Write(buf)

	outbound := readOutbound(stage)

	if !outbound.HasEncryptedPayload() {
		return 0
	}

	return datura.Peek[float64](outbound, "output", "value")
}

func observeInputs(stage io.ReadWriter, series ...float64) float64 {
	return readScalar(stage, series...)
}

func observeWithWork(stage io.ReadWriter, sample float64, work float64) float64 {
	return readScalar(stage, sample, work)
}
