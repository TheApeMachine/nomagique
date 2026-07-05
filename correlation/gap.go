package correlation

import (
	"math"
	"time"

	"gonum.org/v1/gonum/stat"
)

const gapPayloadHeader = 2

func gapSegments(batch []float64) (syncLeft, syncRight, asyncLeft, asyncRight []float64, ok bool) {
	if len(batch) < gapPayloadHeader {
		return nil, nil, nil, nil, false
	}

	syncCount := int(batch[0])
	asyncPairCount := int(batch[1])
	offset := gapPayloadHeader

	if syncCount < 2 || asyncPairCount < 2 {
		return nil, nil, nil, nil, false
	}

	asyncScalarCount := asyncPairCount * 2

	if offset+syncCount+syncCount+asyncScalarCount+asyncScalarCount > len(batch) {
		return nil, nil, nil, nil, false
	}

	syncLeft = batch[offset : offset+syncCount]
	offset += syncCount
	syncRight = batch[offset : offset+syncCount]
	offset += syncCount
	asyncLeft = batch[offset : offset+asyncScalarCount]
	offset += asyncScalarCount
	asyncRight = batch[offset : offset+asyncScalarCount]

	return syncLeft, syncRight, asyncLeft, asyncRight, true
}

func gapPearson(batch []float64, weights []float64) (float64, bool) {
	syncLeft, syncRight, _, _, ok := gapSegments(batch)

	if !ok || len(syncLeft) < 2 || len(syncLeft) != len(syncRight) {
		return 0, false
	}

	weightsOK := len(weights) == 0 || len(weights) == len(syncLeft)

	if !weightsOK {
		return 0, false
	}

	sampleWeights := weights

	if len(sampleWeights) == 0 {
		sampleWeights = nil
	}

	correlationValue := stat.Correlation(syncLeft, syncRight, sampleWeights)

	if math.IsNaN(correlationValue) || math.IsInf(correlationValue, 0) {
		return 0, false
	}

	return correlationValue, true
}

func gapHayashi(batch []float64, maxInterval time.Duration) (float64, bool) {
	_, _, asyncLeft, asyncRight, ok := gapSegments(batch)

	if !ok {
		return 0, false
	}

	left, leftOK := samplesFromScalars(asyncLeft)
	right, rightOK := samplesFromScalars(asyncRight)

	if !leftOK || !rightOK {
		return 0, false
	}

	return hayashiYoshidaCorrelation(left, right, maxInterval)
}

type GapErrorType string

const (
	GapErrorRequireBatch         GapErrorType = "require gap batch payload"
	GapErrorRequireSyncSegments  GapErrorType = "require valid synchronous segments"
	GapErrorRequireAsyncSegments GapErrorType = "require valid asynchronous segments"
	GapErrorNonFiniteGap         GapErrorType = "require finite gap"
)

type GapError string

func (gapError GapError) Error() string {
	return string(gapError)
}
