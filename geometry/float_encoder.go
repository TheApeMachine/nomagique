package geometry

import (
	"math"
	"math/cmplx"
	"sort"
	"sync"
)

/*
FloatEncoder maps raw continuous float vectors into PhaseDials without
going through the lossy Value→structuralPhaseMix XOR hash. Each named
dimension is z-score normalized against its own running history so that
heterogeneous scales (Reynolds ~10³ vs asymmetry ~[-1,1]) don't let
a single axis dominate the phase angle.

The encoder is safe for concurrent use. Each dimension maintains a
Welford online accumulator so normalization adapts as the market
evolves, with no fixed thresholds or magic numbers.
*/
type FloatEncoder struct {
	mu         sync.RWMutex
	dimensions map[string]*welfordAccumulator
}

/*
welfordAccumulator implements Welford's online algorithm for streaming
mean and variance with O(1) memory per dimension.
*/
type welfordAccumulator struct {
	count    uint64
	mean     float64
	variance float64
}

/*
NewFloatEncoder creates a float encoder with empty dimension histories.
*/
func NewFloatEncoder() *FloatEncoder {
	return &FloatEncoder{
		dimensions: make(map[string]*welfordAccumulator),
	}
}

/*
Encode converts a named float vector into a unit-normalized PhaseDial.
Each float is z-score normalized against its dimension's running history
before being mapped to phase. The key order is sorted for determinism.

The returned PhaseDial is unit-normalized and directly compatible with
PhaseDial.Similarity for cosine-based retrieval.
*/
func (encoder *FloatEncoder) Encode(features map[string]float64) PhaseDial {
	if len(features) == 0 {
		return NewPhaseDial()
	}

	keys := make([]string, 0, len(features))

	for key := range features {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	normalized := make([]float64, len(keys))

	encoder.mu.RLock()

	for index, key := range keys {
		value := features[key]
		accumulator := encoder.dimensions[key]

		if accumulator == nil || accumulator.count < 2 {
			normalized[index] = scalarUnitFloat(value)
			continue
		}

		stddev := math.Sqrt(accumulator.variance)

		if stddev < 1e-15 {
			normalized[index] = 0.5
			continue
		}

		zScore := (value - accumulator.mean) / stddev
		normalized[index] = 0.5 + math.Atan(zScore)/math.Pi
	}

	encoder.mu.RUnlock()

	return encodeNormalizedVector(normalized)
}

/*
Update feeds a named float vector into the running statistics for each
dimension. Call this after Encode to update the normalization baseline.
*/
func (encoder *FloatEncoder) Update(features map[string]float64) {
	encoder.mu.Lock()
	defer encoder.mu.Unlock()

	for key, value := range features {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}

		accumulator := encoder.dimensions[key]

		if accumulator == nil {
			accumulator = &welfordAccumulator{}
			encoder.dimensions[key] = accumulator
		}

		accumulator.observe(value)
	}
}

/*
DimensionCount returns the number of tracked dimensions.
*/
func (encoder *FloatEncoder) DimensionCount() int {
	encoder.mu.RLock()
	defer encoder.mu.RUnlock()

	return len(encoder.dimensions)
}

func (accumulator *welfordAccumulator) observe(value float64) {
	accumulator.count++
	delta := value - accumulator.mean
	accumulator.mean += delta / float64(accumulator.count)
	delta2 := value - accumulator.mean

	if accumulator.count > 1 {
		accumulator.variance += (delta*delta2 - accumulator.variance) / float64(accumulator.count)
	}
}

/*
encodeNormalizedVector takes values in [0,1] and maps them into a
PhaseDial using the same prime-frequency phase law as EncodeFromValues,
but without the Value word-structure indirection.

Each normalized feature contributes phase at each prime frequency,
producing a deterministic PhaseDial that preserves the continuous
magnitude relationships between features.
*/
func encodeNormalizedVector(normalized []float64) PhaseDial {
	dial := NewPhaseDial()
	featureCount := len(normalized)

	if featureCount == 0 {
		return dial
	}

	phases := make([]float64, featureCount)

	for index := range normalized {
		phases[index] = normalized[index] * 2 * math.Pi
	}

	for dimIndex := 0; dimIndex < PhaseDialDimensions; dimIndex++ {
		var sum complex128

		omega := float64(PhaseDialPrimes[dimIndex])

		for featureIndex, phase := range phases {
			compositePhase := (omega * float64(featureIndex+1) * 0.1) + phase
			sum += cmplx.Rect(1.0, compositePhase)
		}

		dial[dimIndex] = sum
	}

	return dial.normalize()
}

/*
scalarUnitFloat maps an arbitrary float to [0,1] using arctan compression.
Values already in [0,1] pass through unchanged.
*/
func scalarUnitFloat(value float64) float64 {
	if value >= 0 && value <= 1 {
		return value
	}

	return 0.5 + math.Atan(value)/math.Pi
}
