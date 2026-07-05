package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/hawkes"
)

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
	frame := make([]byte, 0, len(chunk))

	for {
		readCount, err := stage.Read(chunk)

		if readCount > 0 {
			frame = append(frame, chunk[:readCount]...)
		}

		if err == io.EOF {
			break
		}

		if err != nil && err != io.ErrShortBuffer {
			return nil, errnie.Error(errnie.Err(errnie.IO, "readOutbound: stage read failed", err))
		}

		if readCount == 0 {
			break
		}
	}

	if len(frame) == 0 {
		return nil, errnie.Error(errnie.Err(errnie.Validation, "readOutbound: stage produced no output", nil))
	}

	outbound := datura.Acquire("test-out", datura.Artifact_Type_json)
	_, err := outbound.Unpack(frame)

	if err != nil {
		outbound.Release()

		return nil, errnie.Error(errnie.Err(errnie.IO, "readOutbound: outbound write failed", err))
	}

	if !outbound.HasPayload() {
		outbound.Release()

		return nil, errnie.Error(errnie.Err(errnie.Validation, "readOutbound: stage produced no output", nil))
	}

	return outbound, nil
}
