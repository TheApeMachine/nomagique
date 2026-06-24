package hawkes

import (
	"encoding/binary"
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

func momentSamples(wire, config *datura.Artifact) (xValues, yValues, weights []float64, ok bool) {
	features := streamFeatures(wire)

	if len(features) == 0 {
		return nil, nil, nil, false
	}

	xCount, yCount := streamCounts(wire, config, features)

	if xCount > 0 && yCount > 0 && len(features) >= xCount+yCount {
		xValues = append([]float64(nil), features[:xCount]...)
		yValues = append([]float64(nil), features[xCount:xCount+yCount]...)
	} else if len(features) < 4 || len(features)%2 != 0 {
		return nil, nil, nil, false
	} else {
		half := len(features) / 2
		xValues = append([]float64(nil), features[:half]...)
		yValues = append([]float64(nil), features[half:]...)
	}

	weights = datura.Peek[[]float64](wire, "config", "weights")

	if len(weights) == 0 {
		weights = datura.Peek[[]float64](config, "config", "weights")
	}

	if len(weights) == 0 {
		weights = nil
	}

	ok = len(xValues) == len(yValues) && len(xValues) >= 2

	if len(weights) != 0 && len(weights) != len(xValues) {
		ok = false
	}

	return xValues, yValues, weights, ok
}

func fitTimes(wire, config *datura.Artifact) (xTimes, yTimes []float64, ok bool) {
	features := streamFeatures(wire)

	if len(features) == 0 {
		return nil, nil, false
	}

	xCount, yCount := streamCounts(wire, config, features)

	if xCount <= 0 || yCount <= 0 || len(features) < xCount+yCount {
		return nil, nil, false
	}

	xTimes = features[:xCount]
	yTimes = features[xCount : xCount+yCount]

	for _, sample := range append(xTimes, yTimes...) {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return nil, nil, false
		}
	}

	return xTimes, yTimes, len(xTimes)+len(yTimes) >= 2
}

func streamFeatures(wire *datura.Artifact) []float64 {
	features := equation.Features(wire)

	if len(features) > 0 {
		return features
	}

	features = fitFloatBatch(wire)

	if len(features) > 0 {
		return features
	}

	return datura.Peek[[]float64](wire, "batch")
}

func streamCounts(wire, config *datura.Artifact, features []float64) (int, int) {
	xCount := int(countField(wire, "xCount"))
	yCount := int(countField(wire, "yCount"))

	if xCount <= 0 {
		xCount = int(datura.Peek[float64](wire, "config", "xCount"))
	}

	if yCount <= 0 {
		yCount = int(datura.Peek[float64](wire, "config", "yCount"))
	}

	if xCount <= 0 {
		xCount = int(datura.Peek[float64](config, "config", "xCount"))
	}

	if yCount <= 0 {
		yCount = int(datura.Peek[float64](config, "config", "yCount"))
	}

	if xCount <= 0 && yCount <= 0 && len(features) >= 4 && len(features)%2 == 0 {
		xCount = len(features) / 2
		yCount = len(features) / 2
	}

	return xCount, yCount
}

func countField(wire *datura.Artifact, key string) float64 {
	value, err := statistic.FeatureColumn(wire, key)

	if err != nil {
		return 0
	}

	return value
}

func fitFloatBatch(artifact *datura.Artifact) []float64 {
	if !artifact.HasEncryptedPayload() {
		return nil
	}

	payload := artifact.DecryptPayload()

	if len(payload) == 0 || len(payload)%8 != 0 {
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

func fitHorizon(artifact *datura.Artifact) time.Time {
	horizonNano := int64(datura.Peek[float64](artifact, "config", "horizonUnixNano"))

	return time.Unix(0, horizonNano)
}

func bivariateParamsFromArtifact(artifact *datura.Artifact) BivariateParams {
	return BivariateParams{
		MuX:     datura.Peek[float64](artifact, "config", "muX"),
		MuY:     datura.Peek[float64](artifact, "config", "muY"),
		AlphaXX: datura.Peek[float64](artifact, "config", "alphaXX"),
		AlphaXY: datura.Peek[float64](artifact, "config", "alphaXY"),
		AlphaYX: datura.Peek[float64](artifact, "config", "alphaYX"),
		AlphaYY: datura.Peek[float64](artifact, "config", "alphaYY"),
		Beta:    datura.Peek[float64](artifact, "config", "beta"),
	}
}

func bivariateFitFromArtifact(artifact *datura.Artifact) BivariateFit {
	return BivariateFit{
		MuX:            datura.Peek[float64](artifact, "config", "muX"),
		MuY:            datura.Peek[float64](artifact, "config", "muY"),
		AlphaXX:        datura.Peek[float64](artifact, "config", "alphaXX"),
		AlphaXY:        datura.Peek[float64](artifact, "config", "alphaXY"),
		AlphaYX:        datura.Peek[float64](artifact, "config", "alphaYX"),
		AlphaYY:        datura.Peek[float64](artifact, "config", "alphaYY"),
		Beta:           datura.Peek[float64](artifact, "config", "beta"),
		IntensityX:     datura.Peek[float64](artifact, "config", "intensityX"),
		IntensityY:     datura.Peek[float64](artifact, "config", "intensityY"),
		SpectralRadius: datura.Peek[float64](artifact, "config", "spectralRadius"),
	}
}

func fitTimesToTime(samples []float64) []time.Time {
	times := make([]time.Time, len(samples))

	for index, sample := range samples {
		times[index] = time.Unix(0, int64(sample))
	}

	return times
}

func EncodeMomentBatch(xStream, yStream []float64) []float64 {
	if len(xStream) != len(yStream) || len(xStream) < 2 {
		return nil
	}

	batch := make([]float64, 0, len(xStream)+len(yStream))
	batch = append(batch, xStream...)
	batch = append(batch, yStream...)

	return batch
}

func EncodeFitBatch(xTimes, yTimes []float64) []float64 {
	if len(xTimes)+len(yTimes) < 2 {
		return nil
	}

	batch := make([]float64, 0, len(xTimes)+len(yTimes))
	batch = append(batch, xTimes...)
	batch = append(batch, yTimes...)

	return batch
}
