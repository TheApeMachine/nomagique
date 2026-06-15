package tests

import (
	"encoding/binary"
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
RunObserveSampleSequence feeds samples through observeSample and returns the last value.
*/
func RunObserveSampleSequence(observeSample func(sample float64) float64, samples []float64) float64 {
	var last float64

	for _, sample := range samples {
		last = observeSample(sample)
	}

	return last
}

/*
SampleArtifact returns an artifact whose payload holds one float64 sample.
*/
func SampleArtifact(sample float64) *datura.Artifact {
	inbound := datura.Acquire("test-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)

	return inbound
}

/*
SampleArtifactBytes marshals one float64 sample into artifact wire bytes.
*/
func SampleArtifactBytes(sample float64) ([]byte, error) {
	return SampleArtifact(sample).Message().Marshal()
}

/*
WriteSample marshals sample into an artifact and writes it into stage.
*/
func WriteSample(stage io.Writer, sample float64) error {
	inbound := datura.Acquire("test-in", datura.Artifact_Type_json)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(sample))
	_ = inbound.SetPayload(payload)
	buf, err := inbound.Message().Marshal()

	if err != nil {
		return err
	}

	_, err = stage.Write(buf)

	return err
}

/*
WriteSamples marshals multiple float64 samples into one artifact payload.
*/
func WriteSamples(stage io.Writer, samples ...float64) error {
	inbound := datura.Acquire("test-in", datura.Artifact_Type_json)
	payload := encodeFloat64s(samples...)
	_ = inbound.SetPayload(payload)
	buf, err := inbound.Message().Marshal()

	if err != nil {
		return err
	}

	_, err = stage.Write(buf)

	return err
}

/*
ReadSample reads one float64 from stage output.
*/
func ReadSample(stage io.Reader) (float64, error) {
	buf := make([]byte, 4096)
	readCount, err := stage.Read(buf)

	if err != nil && err != io.EOF && err != io.ErrShortBuffer {
		return 0, err
	}

	outbound := datura.Acquire("test-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(buf[:readCount])
	payload, payloadErr := outbound.Payload()

	if payloadErr != nil || len(payload) < 8 {
		return 0, payloadErr
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload)), nil
}

/*
PipelineSample writes sample through stages via io.Copy and reads the final float64.
*/
func PipelineSample(stages []io.ReadWriter, sample float64) (float64, error) {
	if len(stages) == 0 {
		return sample, nil
	}

	if err := WriteSample(stages[0], sample); err != nil {
		return 0, err
	}

	for index := 0; index < len(stages)-1; index++ {
		_, err := io.Copy(stages[index+1], stages[index])

		if err != nil && err != io.EOF {
			return 0, err
		}
	}

	return ReadSample(stages[len(stages)-1])
}

/*
PipelineSeries runs each sample through stages and collects outputs.
*/
func PipelineSeries(stages []io.ReadWriter, samples []float64) ([]float64, error) {
	outputs := make([]float64, len(samples))

	for index, sample := range samples {
		value, err := PipelineSample(stages, sample)

		if err != nil {
			return nil, err
		}

		outputs[index] = value
	}

	return outputs, nil
}

/*
AlmostEqual compares floats with relative tolerance, treating NaN pairs as equal.
*/
func AlmostEqual(left float64, right float64) bool {
	if math.IsNaN(left) && math.IsNaN(right) {
		return true
	}

	return math.Abs(left-right) <= 1e-12*math.Max(1, math.Max(math.Abs(left), math.Abs(right)))
}

func encodeFloat64s(samples ...float64) []byte {
	payload := make([]byte, 8*len(samples))

	for index, sample := range samples {
		offset := index * 8
		binary.BigEndian.PutUint64(payload[offset:offset+8], math.Float64bits(sample))
	}

	return payload
}
