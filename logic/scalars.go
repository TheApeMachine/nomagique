package logic

import (
	"bytes"
	"io"
	"math"

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
		artifact: datura.Acquire("constant", datura.APPJSON).RetainStageAttributes(),
		value:    value,
	}
}

func (constant *Constant) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](constant.artifact, "output") == nil

	constant.artifact.Clear("sample")

	n, err := constant.artifact.Write(p)

	if bootstrap {
		constant.artifact.Clear("output")
	}

	return n, err
}

func (constant *Constant) Read(p []byte) (int, error) {
	constant.artifact.Poke(datura.Map[float64]{"value": constant.value}, "output")

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
