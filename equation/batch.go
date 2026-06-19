package equation

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
)

/*
FloatBatch decodes a little-endian float64 batch from the merged artifact payload.
*/
func FloatBatch(artifact *datura.Artifact) []float64 {
	payload, ok := artifact.PayloadQuiet()

	if !ok || len(payload) == 0 || len(payload)%8 != 0 {
		return nil
	}

	samples := make([]float64, len(payload)/8)

	for index := range samples {
		offset := index * 8
		value := math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil
		}

		samples[index] = value
	}

	return samples
}
