package tests

import (
	"encoding/binary"
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
WriteSamples marshals multiple float64 samples into one artifact payload.
*/
func WriteSamples(stage io.Writer, samples ...float64) error {
	inbound := datura.Acquire("test-in", datura.APPJSON)
	payload := encodeFloat64s(samples...)
	inbound.WithPayload(payload)
	buf, err := inbound.Message().Marshal()

	if err != nil {
		return err
	}

	_, err = stage.Write(buf)

	return err
}

/*
ReadOutputValue reads the stage output map value field.
*/
func ReadOutputValue(stage io.Reader) (float64, error) {
	buf := make([]byte, 4096)
	readCount, err := stage.Read(buf)

	if err != nil && err != io.EOF && err != io.ErrShortBuffer {
		return 0, err
	}

	outbound := datura.Acquire("test-out", datura.APPJSON)
	_, _ = outbound.Write(buf[:readCount])

	return datura.Peek[float64](outbound, "output", "value"), nil
}

func encodeFloat64s(samples ...float64) []byte {
	payload := make([]byte, 8*len(samples))

	for index, sample := range samples {
		offset := index * 8
		binary.BigEndian.PutUint64(payload[offset:offset+8], math.Float64bits(sample))
	}

	return payload
}
