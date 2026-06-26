package tests

import (
	"bytes"
	"errors"
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

/*
WriteSamples marshals a feature vector into one artifact payload.
*/
func WriteSamples(stage io.Writer, samples ...float64) error {
	inbound := datura.Acquire("test-in", datura.APPJSON)
	inbound.WithPayload(equation.MarshalFeaturesPayload(samples))
	buf := inbound.Pack()

	if len(buf) == 0 {
		return errors.New("tests: artifact pack failed")
	}

	_, err := stage.Write(buf)

	return err
}

/*
ReadOutputValue reads the stage output map value field.
*/
func ReadOutputValue(stage io.Reader) (float64, error) {
	buf := make([]byte, 4096)
	var out bytes.Buffer

	for {
		readCount, err := stage.Read(buf)

		if readCount > 0 {
			out.Write(buf[:readCount])
		}

		if err == io.EOF {
			break
		}

		if err != nil && err != io.ErrShortBuffer {
			return 0, err
		}
	}

	outbound := datura.Acquire("test-out", datura.APPJSON)
	_, _ = outbound.Unpack(out.Bytes())

	return datura.Peek[float64](outbound, "output", "value"), nil
}
