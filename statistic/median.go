package statistic

import (
	"encoding/binary"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Median computes the sample median of a stream of numbers.
*/
type Median struct {
	artifact       *datura.Artifact
	weights        []float64
	panel          *Panel
	excludedKey    float64
	hasExcludedKey bool
}

/*
NewMedian creates a median stage.

When panel is non-nil, the payload carries the member key to exclude and the
median is computed over the remaining panel peers. Otherwise the artifact
payload supplies the sample batch.
*/
func NewMedian(weights []float64, panel *Panel) *Median {
	return &Median{
		artifact: datura.Acquire("median", datura.Artifact_Type_json),
		weights:  weights,
		panel:    panel,
	}
}

func (median *Median) Write(p []byte) (int, error) {
	route := datura.Acquire("median-route", datura.Artifact_Type_json)
	_, _ = route.Write(p)
	payload, payloadErr := route.Payload()

	if payloadErr == nil && len(payload) == 8 {
		median.excludedKey = math.Float64frombits(binary.BigEndian.Uint64(payload))
		median.hasExcludedKey = true
	}

	return median.artifact.Write(p)
}

func (median *Median) Read(p []byte) (int, error) {
	if median.panel != nil {
		median.readPanelPeers()

		return median.artifact.Read(p)
	}

	if median.readCoupledPeers() {
		return median.artifact.Read(p)
	}

	payload, err := median.artifact.Payload()

	if err != nil || len(payload) < 8 || len(payload)%8 != 0 {
		return median.artifact.Read(p)
	}

	count := len(payload) / 8
	values := make([]float64, count)

	for index := range count {
		offset := index * 8
		values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
	}

	if len(values) == 0 {
		return median.artifact.Read(p)
	}

	weights := median.weights

	if len(weights) == 0 {
		putFloat64Payload(&median.artifact, "median", medianOf(values))

		return median.artifact.Read(p)
	}

	if len(weights) != len(values) {
		errnie.Err(
			errnie.Validation, "unable to compute median",
			MedianError(MedianErrorWeightLengthMismatch),
		)

		putFloat64Payload(&median.artifact, "median", 0)

		return median.artifact.Read(p)
	}

	putFloat64Payload(&median.artifact, "median", weightedMedian(values, weights))

	return median.artifact.Read(p)
}

func (median *Median) Close() error {
	return nil
}

/*
Value returns the last derived median without re-processing the stage.
*/
func (median *Median) Value() float64 {
	payload, err := median.artifact.Payload()

	if err != nil || len(payload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload))
}

func (median *Median) readPanelPeers() {
	payload, err := median.artifact.Payload()

	if err != nil || len(payload) != 8 {
		return
	}

	excludedKey := math.Float64frombits(binary.BigEndian.Uint64(payload))
	peerSamples := median.panel.peerSamples(excludedKey)

	if len(peerSamples) == 0 {
		putFloat64Payload(&median.artifact, "median", 0)

		return
	}

	putFloat64Payload(&median.artifact, "median", MedianOf(peerSamples))
}

func (median *Median) readCoupledPeers() bool {
	if !median.hasExcludedKey {
		return false
	}

	payload, payloadErr := median.artifact.Payload()

	if payloadErr != nil || len(payload) < 16 || len(payload)%16 != 0 {
		return false
	}

	peerSamples := make([]float64, 0, len(payload)/16)

	for offset := 0; offset < len(payload); offset += 16 {
		memberKey := math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload[offset+8 : offset+16]))

		if memberKey == median.excludedKey {
			continue
		}

		peerSamples = append(peerSamples, sample)
	}

	if len(peerSamples) == 0 {
		putFloat64Payload(&median.artifact, "median", 0)

		return true
	}

	putFloat64Payload(&median.artifact, "median", MedianOf(peerSamples))

	return true
}

func (median *Median) Reset() error {
	median.weights = nil
	median.artifact = datura.Acquire("median", datura.Artifact_Type_json)

	return nil
}

/*
MedianOf returns the median of values without weights.
*/
func MedianOf(values []float64) float64 {
	return medianOf(values)
}

func medianOf(values []float64) float64 {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return math.NaN()
		}
	}

	sort.Float64s(values)

	middle := len(values) / 2

	if len(values)%2 == 1 {
		return values[middle]
	}

	return (values[middle-1] + values[middle]) / 2
}

func weightedMedian(values, weights []float64) float64 {
	sortedValues, sortedWeights, ok := sortWeightedSamples(values, weights)

	if !ok {
		return math.NaN()
	}

	return stat.Quantile(0.5, stat.Empirical, sortedValues, sortedWeights)
}

type MedianErrorType string

const (
	MedianErrorWeightLengthMismatch MedianErrorType = "require equal weight length"
)

type MedianError string

func (medianError MedianError) Error() string {
	return string(medianError)
}
