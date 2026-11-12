package correlation

import (
	"math"
	"time"

	"gonum.org/v1/gonum/stat"
)

/*
gapPayloadHeader is the leading scalar count pair in a gap batch payload
before synchronous and asynchronous segment slices are laid out.
*/
const gapPayloadHeader = 2

/*
gapSegments decodes the packed batch layout; malformed lengths return ok false
so Pearson and Hayashi paths fail closed instead of reading past the buffer.
*/
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

/*
gapPearson estimates synchronous gap correlation when aligned segments exist;
non-finite gonum output is discarded so downstream gates see a hard false.
*/
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

/*
gapHayashi runs asynchronous Hayashi–Yoshida on the decoded async segments
when sync layout is present but irregular sampling drives the estimate.
*/
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

/*
GapErrorType categorizes gap-batch validation failures for typed error routing.
*/
type GapErrorType string

const (
	GapErrorRequireBatch         GapErrorType = "require gap batch payload"
	GapErrorRequireSyncSegments  GapErrorType = "require valid synchronous segments"
	GapErrorRequireAsyncSegments GapErrorType = "require valid asynchronous segments"
	GapErrorNonFiniteGap         GapErrorType = "require finite gap"
)

/*
GapError is a string error for gap correlation preconditions.
*/
type GapError string

var _ error = (*GapError)(nil)

/*
Error satisfies the error interface for GapError constants.
*/
func (gapError GapError) Error() string {
	return string(gapError)
}
