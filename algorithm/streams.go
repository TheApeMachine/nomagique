package algorithm

import (
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/theapemachine/nomagique/correlation"
)

var (
	ErrEmptyInputs = errors.New("algorithm: empty inputs")
)

func payloadSamples(payload []byte) []float64 {
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

	value := math.Float64frombits(binary.BigEndian.Uint64(payload))

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}

	return value, true
}

func zipNodeRows(streams [][]float64) ([][]float64, bool) {
	if len(streams) == 0 {
		return nil, false
	}

	length := len(streams[0])

	if length == 0 {
		return nil, false
	}

	for _, stream := range streams {
		if len(stream) != length {
			return nil, false
		}
	}

	rows := make([][]float64, length)

	for index := range rows {
		rows[index] = make([]float64, len(streams))

		for nodeIndex, stream := range streams {
			rows[index][nodeIndex] = stream[index]
		}
	}

	return rows, true
}

func samplesFromTimeValues(values []float64) ([]correlation.Sample, bool) {
	if len(values) < 4 || len(values)%2 != 0 {
		return nil, false
	}

	samples := make([]correlation.Sample, len(values)/2)

	for index := range samples {
		pair := index * 2
		seconds := values[pair]
		value := values[pair+1]

		if math.IsNaN(seconds) || math.IsInf(seconds, 0) ||
			math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}

		wholeSeconds := int64(seconds)
		nanoseconds := int64((seconds - float64(wholeSeconds)) * float64(time.Second))

		samples[index] = correlation.Sample{
			At:    time.Unix(wholeSeconds, nanoseconds),
			Value: value,
		}
	}

	return samples, true
}

func hayashiCorrelation(
	left, right []correlation.Sample, maxInterval time.Duration,
) (float64, bool) {
	if len(left) < 2 || len(right) < 2 {
		return 0, false
	}

	leftVariance := hayashiVarianceSum(left, maxInterval)
	rightVariance := hayashiVarianceSum(right, maxInterval)

	if leftVariance <= 0 || rightVariance <= 0 {
		return 0, false
	}

	covariance := 0.0
	rightStart := 0

	for leftIndex := 0; leftIndex < len(left)-1; leftIndex++ {
		leftStart := left[leftIndex].At
		leftEnd := left[leftIndex+1].At

		if !validHayashiInterval(left[leftIndex], left[leftIndex+1], maxInterval) {
			continue
		}

		leftReturn := math.Log(left[leftIndex+1].Value / left[leftIndex].Value)

		for rightStart < len(right)-1 {
			if !validHayashiInterval(right[rightStart], right[rightStart+1], maxInterval) ||
				!leftStart.Before(right[rightStart+1].At) {
				rightStart++

				continue
			}

			break
		}

		for rightIndex := rightStart; rightIndex < len(right)-1; rightIndex++ {
			rightIntervalStart := right[rightIndex].At

			if !rightIntervalStart.Before(leftEnd) {
				break
			}

			if !validHayashiInterval(right[rightIndex], right[rightIndex+1], maxInterval) {
				continue
			}

			covariance += leftReturn * math.Log(
				right[rightIndex+1].Value/right[rightIndex].Value,
			)
		}
	}

	denominator := math.Sqrt(leftVariance * rightVariance)

	if denominator <= 0 {
		return 0, false
	}

	correlationValue := covariance / denominator

	if correlationValue > 1 {
		return 1, true
	}

	if correlationValue < -1 {
		return -1, true
	}

	return correlationValue, true
}

func hayashiVarianceSum(samples []correlation.Sample, maxInterval time.Duration) float64 {
	if len(samples) < 2 {
		return 0
	}

	sum := 0.0

	for index := 1; index < len(samples); index++ {
		if !validHayashiInterval(samples[index-1], samples[index], maxInterval) {
			continue
		}

		ret := math.Log(samples[index].Value / samples[index-1].Value)
		sum += ret * ret
	}

	return sum
}

func validHayashiInterval(
	previous, current correlation.Sample, maxInterval time.Duration,
) bool {
	if previous.Value <= 0 || current.Value <= 0 || !previous.At.Before(current.At) {
		return false
	}

	if maxInterval <= 0 {
		return true
	}

	return current.At.Sub(previous.At) <= maxInterval
}

func appendRingFloat(values []float64, value float64, capacity int) []float64 {
	values = append(values, value)

	if len(values) <= capacity {
		return values
	}

	return values[len(values)-capacity:]
}
