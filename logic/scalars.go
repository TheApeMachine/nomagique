package logic

import (
	"bytes"
	"io"
	"math"

	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
)

func boundarySample(artifact *datura.Artifact) (float64, bool) {
	sample := datura.Peek[float64](artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, false
	}

	return sample, true
}

func artifactBytes(artifact *datura.Artifact) ([]byte, bool) {
	buf, err := artifact.MarshalPacked()

	if err != nil {
		return nil, false
	}

	return buf, true
}

func readOperand(stage io.ReadWriteCloser, artifactBytes []byte) (float64, bool) {
	if stage == nil {
		return 0, false
	}

	if constant, isConstant := stage.(*Constant); isConstant {
		return constant.value, true
	}

	_, writeErr := stage.Write(artifactBytes)

	if writeErr != nil {
		return 0, false
	}

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
			return 0, false
		}
	}

	outbound := datura.Acquire("logic-operand", datura.APPJSON)
	_, _ = outbound.Write(outBuf.Bytes())

	value := datura.Peek[float64](outbound, "output", "value")

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}

	return value, true
}

/*
Constant emits a fixed scalar on every Read.
*/
type Constant struct {
	artifact *datura.Artifact
	value    float64
}

func NewConstant(value float64) *Constant {
	return &Constant{
		artifact: datura.Acquire("constant", datura.APPJSON),
		value:    value,
	}
}

func (constant *Constant) Write(p []byte) (int, error) {
	constant.artifact.WithPayload(p)
	return len(p), nil
}

func (constant *Constant) Read(p []byte) (int, error) {
	state := datura.Acquire("constant-state", datura.APPJSON)

	if _, err := state.Write(constant.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.MergeOutput("value", constant.value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(p)
}

func (constant *Constant) Close() error {
	return nil
}

func attributeKeyPresent(artifact *datura.Artifact, key string) bool {
	raw, err := artifact.Attributes()

	if err != nil || len(raw) == 0 {
		return false
	}

	_, getErr := sonic.Get(raw, key)

	return getErr == nil
}

func inboundReset(p []byte) bool {
	inbound := datura.Acquire("inbound", datura.APPJSON)
	_, _ = inbound.Write(p)

	return attributeKeyPresent(inbound, "reset")
}

func writeResetToStage(stage io.ReadWriteCloser) {
	if stage == nil {
		return
	}

	resetArtifact, resetOK := artifactBytes(
		datura.Acquire("logic-reset", datura.APPJSON).Poke(1, "reset"),
	)

	if !resetOK {
		return
	}

	_, _ = stage.Write(resetArtifact)
}

func resetCondition(condition Condition) {
	if condition == nil {
		return
	}

	condition.ResetOperands()
}
