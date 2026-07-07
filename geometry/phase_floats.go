package geometry

import (
	"math"
	"math/cmplx"
)

/*
EncodeFromFloats builds a PhaseDial directly from a continuous feature vector,
preserving each feature's magnitude as phase instead of hashing it through a
Value slab. Feature i drives dimension k by omega_k*(i+1)*0.1 + feature_i*2π, so
two states with close normalized features encode to close dials and Similarity
then measures how near the market states actually are — the whole point of
routing continuously rather than collapsing to a category label. Features must be
normalized to [0,1] so magnitude maps to phase without wrap-around dominating.
*/
func (dial PhaseDial) EncodeFromFloats(features []float64) PhaseDial {
	if len(features) == 0 {
		return dial
	}

	if len(dial) < PhaseDialDimensions {
		dial = NewPhaseDial()
	}

	for dimIndex := 0; dimIndex < PhaseDialDimensions; dimIndex++ {
		var sum complex128

		omega := float64(PhaseDialPrimes[dimIndex])

		for featureIndex, feature := range features {
			phase := (omega * float64(featureIndex+1) * 0.1) + (feature * 2 * math.Pi)
			sum += cmplx.Rect(1.0, phase)
		}

		dial[dimIndex] = sum
	}

	return dial.normalize()
}
