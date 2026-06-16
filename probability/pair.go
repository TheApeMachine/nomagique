package probability

import (
	"encoding/binary"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/theapemachine/datura"
)

var (
	ErrEmptyInputs    = errors.New("probability: empty inputs")
	ErrZeroPredicted  = errors.New("probability: zero predicted value")
	ErrInvalidOutcome = errors.New("probability: invalid Bernoulli outcome")
)

func payloadSamples(payload []byte) []float64 {
	if len(payload) == 0 || len(payload)%8 != 0 {
		return nil
	}

	samples := make([]float64, len(payload)/8)

	for index := range samples {
		offset := index * 8
		samples[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
	}

	return samples
}

func encodePayload(samples ...float64) []byte {
	payload := make([]byte, 8*len(samples))

	for index, sample := range samples {
		offset := index * 8
		binary.BigEndian.PutUint64(payload[offset:offset+8], math.Float64bits(sample))
	}

	return payload
}

func payloadScalar(payload []byte) (float64, bool) {
	if len(payload) != 8 {
		return 0, false
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload)), true
}

func pokeFloat(artifact *datura.Artifact, key string, value float64) {
	artifact.Poke(key, strconv.FormatFloat(value, 'g', -1, 64))
}

func pokeInt(artifact *datura.Artifact, key string, value int) {
	artifact.Poke(key, strconv.Itoa(value))
}

func pokeFloatList(artifact *datura.Artifact, key string, values []float64) {
	if len(values) == 0 {
		return
	}

	parts := make([]string, len(values))

	for index, value := range values {
		parts[index] = strconv.FormatFloat(value, 'g', -1, 64)
	}

	artifact.Poke(key, strings.Join(parts, ","))
}

func rehydrateArtifact(artifact **datura.Artifact, origin string, artifactType datura.Artifact_Type) {
	if artifact == nil || *artifact == nil {
		return
	}

	current := *artifact
	payload, _ := current.Payload()
	fresh := datura.Acquire(origin, artifactType)

	if fresh == nil {
		return
	}

	if len(payload) > 0 {
		_ = fresh.SetPayload(payload)
	}

	attrs, err := current.Attributes()

	if err == nil {
		for index := 0; index < attrs.Len(); index++ {
			attr := attrs.At(index)
			key, keyErr := attr.Key()
			value, valueErr := attr.Value()

			if keyErr != nil || valueErr != nil {
				continue
			}

			fresh.Poke(key, value)
		}
	}

	*artifact = fresh
}

func parsePredictedActual(
	primary float64, extras []float64,
) (float64, float64, error) {
	if len(extras) >= 2 {
		predicted := extras[0]
		actual := extras[1]

		if predicted == 0 {
			return 0, 0, ErrZeroPredicted
		}

		return predicted, actual, nil
	}

	if len(extras) == 0 {
		return 0, 0, ErrEmptyInputs
	}

	predicted := primary
	actual := extras[0]

	if predicted == 0 {
		return 0, 0, ErrZeroPredicted
	}

	return predicted, actual, nil
}

func parseBernoulliOutcome(primary float64, extras []float64) (float64, error) {
	if len(extras) > 0 {
		predicted, actual, err := parsePredictedActual(primary, extras)

		if err != nil {
			return 0, err
		}

		if actual >= predicted {
			return 1, nil
		}

		return 0, nil
	}

	outcome := primary

	if outcome < 0 || outcome > 1 {
		return 0, ErrInvalidOutcome
	}

	return outcome, nil
}
