package logic

import (
	"encoding/binary"
	"io"
	"math"

	"github.com/theapemachine/datura"
)

func float64Batch(artifact *datura.Artifact) []float64 {
	if artifact == nil {
		return nil
	}

	payload, err := artifact.Payload()

	if err != nil || len(payload) < 8 || len(payload)%8 != 0 {
		return nil
	}

	count := len(payload) / 8
	values := make([]float64, count)

	for index := range count {
		offset := index * 8
		values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
	}

	return values
}

func boundarySample(artifact *datura.Artifact) (float64, bool) {
	values := float64Batch(artifact)

	if len(values) == 0 {
		return 0, false
	}

	return values[0], true
}

func artifactBytes(artifact *datura.Artifact) ([]byte, bool) {
	buf, err := artifact.Message().Marshal()

	if err != nil {
		return nil, false
	}

	return buf, true
}

func readOperand(stage io.ReadWriter, artifactBytes []byte) (float64, bool) {
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

	outBuf := make([]byte, 4096)
	readCount, readErr := stage.Read(outBuf)

	if readErr != nil && readErr != io.EOF && readErr != io.ErrShortBuffer {
		return 0, false
	}

	outbound := datura.Acquire("logic-operand", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf[:readCount])
	payload, payloadErr := outbound.Payload()

	if payloadErr != nil || len(payload) != 8 {
		return 0, false
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload)), true
}

func putFloat64Payload(artifact **datura.Artifact, name string, value float64) {
	*artifact = datura.Acquire(name, datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(value))
	_ = (*artifact).SetPayload(payload)
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
		artifact: datura.Acquire("constant", datura.Artifact_Type_json),
		value:    value,
	}
}

func (constant *Constant) Write(p []byte) (int, error) {
	return constant.artifact.Write(p)
}

func (constant *Constant) Read(p []byte) (int, error) {
	putFloat64Payload(&constant.artifact, "constant", constant.value)

	return constant.artifact.Read(p)
}

func (constant *Constant) Close() error {
	return nil
}

func (constant *Constant) Reset() error {
	return nil
}

type resetter interface {
	Reset() error
}

func resetStage(stage io.ReadWriter) error {
	if stage == nil {
		return nil
	}

	stageResetter, ok := stage.(resetter)

	if !ok {
		return nil
	}

	return stageResetter.Reset()
}
