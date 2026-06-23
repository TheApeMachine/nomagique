package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/hawkes"
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

func hawkesMomentConfig(params hawkes.BivariateParams, momentR, momentS float64) *datura.Artifact {
	return datura.Acquire("hawkes-moment-config", datura.APPJSON).
		Poke(params.MuX, "config", "muX").
		Poke(params.MuY, "config", "muY").
		Poke(params.AlphaXX, "config", "alphaXX").
		Poke(params.AlphaXY, "config", "alphaXY").
		Poke(params.AlphaYX, "config", "alphaYX").
		Poke(params.AlphaYY, "config", "alphaYY").
		Poke(params.Beta, "config", "beta").
		Poke(momentR, "config", "momentR").
		Poke(momentS, "config", "momentS")
}

func hawkesFitConfig(horizonUnixNano float64) *datura.Artifact {
	return datura.Acquire("hawkes-fit-config", datura.APPJSON).
		Poke(horizonUnixNano, "config", "horizonUnixNano")
}

func readOutbound(stage io.Reader) (*datura.Artifact, error) {
	chunk := make([]byte, 262144)
	readCount, readErr := stage.Read(chunk)

	if readErr != nil && readErr != io.EOF && readErr != io.ErrShortBuffer {
		return nil, errnie.Error(errnie.Err(errnie.IO, "readOutbound: stage read failed", readErr))
	}

	outbound := datura.Acquire("test-out", datura.Artifact_Type_json)
	_, writeErr := outbound.Write(chunk[:readCount])

	if writeErr != nil {
		outbound.Release()

		return nil, errnie.Error(errnie.Err(errnie.IO, "readOutbound: outbound write failed", writeErr))
	}

	if !outbound.HasEncryptedPayload() {
		outbound.Release()

		return nil, errnie.Error(errnie.Err(errnie.Validation, "readOutbound: stage produced no output", nil))
	}

	return outbound, nil
}

func readScalar(stage io.ReadWriter, samples ...float64) float64 {
	inbound := datura.Acquire("test-in", datura.Artifact_Type_json)
	inbound.WithPayload(equation.MarshalFeaturesPayload(samples))
	buf := inbound.Pack()

	if len(buf) == 0 {
		return 0
	}

	_, _ = stage.Write(buf)

	outbound, err := readOutbound(stage)

	if err != nil {
		return 0
	}

	defer outbound.Release()

	return datura.Peek[float64](outbound, "output", "value")
}

func observeInputs(stage io.ReadWriter, series ...float64) float64 {
	return readScalar(stage, series...)
}

func observeWithWork(stage io.ReadWriter, sample float64, work float64) float64 {
	return readScalar(stage, sample, work)
}
