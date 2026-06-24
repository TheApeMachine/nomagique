package geometry

import (
	"math"
)

/*
Multivector represents a grade-restricted element of PGA Cl(3,0,1).
Only the even subalgebra (rotor-viable grades 0 and 2, plus pseudoscalar)
is stored, giving 8 float64 components instead of the full 16.

Layout: [scalar, e01, e02, e03, e12, e31, e23, e0123]
*/
type Multivector [8]float64

const (
	MvScalar = iota
	MvE01
	MvE02
	MvE03
	MvE12
	MvE31
	MvE23
	MvE0123
	multivectorComponentCount
)

/*
FromRotation configures the multivector as a rotation rotor.
*/
func (multivector *Multivector) FromRotation(
	angle, axisE12, axisE31, axisE23 float64,
) {
	half := angle / 2
	sinHalf := math.Sin(half)
	axisNorm := math.Sqrt(axisE12*axisE12 + axisE31*axisE31 + axisE23*axisE23)

	if axisNorm > 0 {
		axisE12 /= axisNorm
		axisE31 /= axisNorm
		axisE23 /= axisNorm
	}

	*multivector = Multivector{
		math.Cos(half),
		0,
		0,
		0,
		sinHalf * axisE12,
		sinHalf * axisE31,
		sinHalf * axisE23,
		0,
	}
}

/*
FromTranslation configures the multivector as a translation motor.
*/
func (multivector *Multivector) FromTranslation(dx, dy, dz float64) {
	*multivector = Multivector{
		1,
		dx / 2,
		dy / 2,
		dz / 2,
		0,
		0,
		0,
		0,
	}
}

/*
FromComponents loads all eight even-subalgebra components from scalars.
*/
func (multivector *Multivector) FromComponents(scalars []float64) {
	*multivector = Multivector{}
	limit := len(scalars)

	if limit > multivectorComponentCount {
		limit = multivectorComponentCount
	}

	for index := 0; index < limit; index++ {
		if math.IsNaN(scalars[index]) || math.IsInf(scalars[index], 0) {
			continue
		}

		multivector[index] = scalars[index]
	}
}

/*
GeometricProduct computes the PGA geometric product of two even-subalgebra multivectors.
*/
func (multivector Multivector) GeometricProduct(other Multivector) Multivector {
	return Multivector{
		multivector[0]*other[0] - multivector[4]*other[4] - multivector[5]*other[5] - multivector[6]*other[6],

		multivector[0]*other[1] + multivector[1]*other[0] - multivector[2]*other[4] + multivector[3]*other[5] +
			multivector[4]*other[2] - multivector[5]*other[3] - multivector[6]*other[7] - multivector[7]*other[6],

		multivector[0]*other[2] + multivector[1]*other[4] + multivector[2]*other[0] - multivector[3]*other[6] -
			multivector[4]*other[1] - multivector[5]*other[7] + multivector[6]*other[3] - multivector[7]*other[5],

		multivector[0]*other[3] - multivector[1]*other[5] + multivector[2]*other[6] + multivector[3]*other[0] -
			multivector[4]*other[7] + multivector[5]*other[1] - multivector[6]*other[2] - multivector[7]*other[4],

		multivector[0]*other[4] + multivector[4]*other[0] + multivector[5]*other[6] - multivector[6]*other[5],

		multivector[0]*other[5] - multivector[4]*other[6] + multivector[5]*other[0] + multivector[6]*other[4],

		multivector[0]*other[6] + multivector[4]*other[5] - multivector[5]*other[4] + multivector[6]*other[0],

		multivector[0]*other[7] + multivector[1]*other[6] + multivector[2]*other[5] + multivector[3]*other[4] +
			multivector[4]*other[3] + multivector[5]*other[2] + multivector[6]*other[1] + multivector[7]*other[0],
	}
}

/*
Reverse computes the reverse of a multivector.
*/
func (multivector Multivector) Reverse() Multivector {
	return Multivector{
		multivector[MvScalar],
		-multivector[MvE01],
		-multivector[MvE02],
		-multivector[MvE03],
		-multivector[MvE12],
		-multivector[MvE31],
		-multivector[MvE23],
		multivector[MvE0123],
	}
}

/*
Sandwich computes mv · target · mv†.
*/
func (multivector Multivector) Sandwich(target Multivector) Multivector {
	return multivector.GeometricProduct(target).GeometricProduct(multivector.Reverse())
}

/*
Normalize scales the multivector bulk norm to unity.
*/
func (multivector Multivector) Normalize() Multivector {
	bulkSq := multivector[MvScalar]*multivector[MvScalar] +
		multivector[MvE12]*multivector[MvE12] +
		multivector[MvE31]*multivector[MvE31] +
		multivector[MvE23]*multivector[MvE23]

	if bulkSq <= 0 || math.IsNaN(bulkSq) || math.IsInf(bulkSq, 0) {
		return multivector
	}

	inv := 1.0 / math.Sqrt(bulkSq)

	return Multivector{
		multivector[0] * inv,
		multivector[1] * inv,
		multivector[2] * inv,
		multivector[3] * inv,
		multivector[4] * inv,
		multivector[5] * inv,
		multivector[6] * inv,
		multivector[7] * inv,
	}
}

/*
Compose chains two rotors: result = other · multivector.
*/
func (multivector Multivector) Compose(other Multivector) Multivector {
	return other.GeometricProduct(multivector)
}

func multivectorSlice(multivector Multivector) []float64 {
	slice := make([]float64, multivectorComponentCount)

	for index := range slice {
		slice[index] = multivector[index]
	}

	return slice
}
