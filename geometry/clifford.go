package geometry

import (
	"math"

	"github.com/theapemachine/nomagique/core"
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
	for index := range multivectorComponentCount {
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

	if bulkSq == 0 {
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

/*
Rotor builds a rotation motor from angle and axis bivector scalars.
*/
type Rotor[T ~float64] struct {
	multivector Multivector
	output      core.Scalar[T]
}

/*
NewRotor returns a rotor stage for nomagique.Number pipelines.
*/
func NewRotor[T ~float64]() *Rotor[T] {
	return &Rotor[T]{}
}

/*
Observe expects four scalars: angle, axisE12, axisE31, axisE23.
*/
func (rotor *Rotor[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) < 4 {
		return rotor.output
	}

	rotor.multivector.FromRotation(scalars[0], scalars[1], scalars[2], scalars[3])
	rotor.output = core.Scalar[T](T(rotor.multivector[MvScalar]))

	return rotor.output
}

/*
Multivector returns the rotor built by the last Observe call.
*/
func (rotor *Rotor[T]) Multivector() Multivector {
	return rotor.multivector
}

/*
Reset clears derived state.
*/
func (rotor *Rotor[T]) Reset() error {
	rotor.multivector = Multivector{}
	rotor.output = core.Scalar[T](0)

	return nil
}

/*
Translator builds a translation motor from displacement scalars.
*/
type Translator[T ~float64] struct {
	multivector Multivector
	output      core.Scalar[T]
}

/*
NewTranslator returns a translation stage for nomagique.Number pipelines.
*/
func NewTranslator[T ~float64]() *Translator[T] {
	return &Translator[T]{}
}

/*
Observe expects three scalars: dx, dy, dz.
*/
func (translator *Translator[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) < 3 {
		return translator.output
	}

	translator.multivector.FromTranslation(scalars[0], scalars[1], scalars[2])
	translator.output = core.Scalar[T](T(translator.multivector[MvScalar]))

	return translator.output
}

/*
Multivector returns the motor built by the last Observe call.
*/
func (translator *Translator[T]) Multivector() Multivector {
	return translator.multivector
}

/*
Reset clears derived state.
*/
func (translator *Translator[T]) Reset() error {
	translator.multivector = Multivector{}
	translator.output = core.Scalar[T](0)

	return nil
}

/*
Sandwich applies a configured motor sandwich to an observed multivector.
*/
type Sandwich[T ~float64] struct {
	motor  Multivector
	output core.Scalar[T]
}

/*
NewSandwich returns a sandwich stage bound to motor.
*/
func NewSandwich[T ~float64](motor Multivector) *Sandwich[T] {
	return &Sandwich[T]{
		motor: motor,
	}
}

/*
Observe expects eight target component scalars in layout order.
*/
func (sandwich *Sandwich[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) < multivectorComponentCount {
		return sandwich.output
	}

	var target Multivector

	target.FromComponents(scalars)
	result := sandwich.motor.Sandwich(target)
	sandwich.output = core.Scalar[T](T(result[MvScalar]))

	return sandwich.output
}

/*
Reset clears derived output.
*/
func (sandwich *Sandwich[T]) Reset() error {
	sandwich.output = core.Scalar[T](0)

	return nil
}
