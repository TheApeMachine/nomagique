package tests

import (
	"bytes"
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
	buf, err := inbound.Message().MarshalPacked()

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
	_, _ = outbound.Write(out.Bytes())

	return datura.Peek[float64](outbound, "output", "value"), nil
}
