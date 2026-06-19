package geometry

import (
	"math"

	"github.com/theapemachine/datura"
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

/*
Rotor builds a rotation motor from angle and axis bivector scalars.
*/
type Rotor struct {
	artifact    *datura.Artifact
	multivector Multivector
	output      float64
}

/*
NewRotor returns a rotor stage for nomagique.Number pipelines.
*/
func NewRotor() *Rotor {
	return &Rotor{
		artifact: datura.Acquire("rotor", datura.APPJSON).RetainStageAttributes(),
	}
}

func (rotor *Rotor) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](rotor.artifact, "output") == nil

	rotor.artifact.Clear("batch")

	n, err := rotor.artifact.Write(p)

	if bootstrap {
		rotor.artifact.Clear("output")
	}

	return n, err
}

func (rotor *Rotor) Read(p []byte) (int, error) {
	scalars := datura.Peek[[]float64](rotor.artifact, "batch")

	if len(scalars) >= 4 {
		rotor.multivector.FromRotation(scalars[0], scalars[1], scalars[2], scalars[3])
		rotor.output = rotor.multivector[MvScalar]
		rotor.artifact.Poke(datura.Map[float64]{"value": rotor.output}, "output")
	}

	return rotor.artifact.Read(p)
}

func (rotor *Rotor) Close() error {
	return nil
}

/*
Multivector returns the rotor built by the last Read call.
*/
func (rotor *Rotor) Multivector() Multivector {
	return rotor.multivector
}

func (rotor *Rotor) Reset() error {
	rotor.multivector = Multivector{}
	rotor.output = 0
	rotor.artifact.Clear("output")

	return nil
}

/*
Translator builds a translation motor from displacement scalars.
*/
type Translator struct {
	artifact    *datura.Artifact
	multivector Multivector
	output      float64
}

/*
NewTranslator returns a translation stage for nomagique.Number pipelines.
*/
func NewTranslator() *Translator {
	return &Translator{
		artifact: datura.Acquire("translator", datura.APPJSON).RetainStageAttributes(),
	}
}

func (translator *Translator) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](translator.artifact, "output") == nil

	translator.artifact.Clear("batch")

	n, err := translator.artifact.Write(p)

	if bootstrap {
		translator.artifact.Clear("output")
	}

	return n, err
}

func (translator *Translator) Read(p []byte) (int, error) {
	scalars := datura.Peek[[]float64](translator.artifact, "batch")

	if len(scalars) >= 3 {
		translator.multivector.FromTranslation(scalars[0], scalars[1], scalars[2])
		translator.output = translator.multivector[MvScalar]
		translator.artifact.Poke(datura.Map[float64]{"value": translator.output}, "output")
	}

	return translator.artifact.Read(p)
}

func (translator *Translator) Close() error {
	return nil
}

/*
Multivector returns the motor built by the last Read call.
*/
func (translator *Translator) Multivector() Multivector {
	return translator.multivector
}

func (translator *Translator) Reset() error {
	translator.multivector = Multivector{}
	translator.output = 0
	translator.artifact.Clear("output")

	return nil
}

/*
Sandwich applies a configured motor sandwich to an observed multivector.
*/
type Sandwich struct {
	artifact *datura.Artifact
	motor    Multivector
	output   float64
}

/*
NewSandwich returns a sandwich stage bound to motor.
*/
func NewSandwich(motor Multivector) *Sandwich {
	return &Sandwich{
		artifact: datura.Acquire("sandwich", datura.APPJSON).RetainStageAttributes(),
		motor:    motor,
	}
}

func (sandwich *Sandwich) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](sandwich.artifact, "output") == nil

	sandwich.artifact.Clear("batch")

	n, err := sandwich.artifact.Write(p)

	if bootstrap {
		sandwich.artifact.Clear("output")
	}

	return n, err
}

func (sandwich *Sandwich) Read(p []byte) (int, error) {
	scalars := datura.Peek[[]float64](sandwich.artifact, "batch")

	if len(scalars) >= multivectorComponentCount {
		var target Multivector

		target.FromComponents(scalars)
		result := sandwich.motor.Sandwich(target)
		sandwich.output = result[MvScalar]
		sandwich.artifact.Poke(datura.Map[float64]{"value": sandwich.output}, "output")
	}

	return sandwich.artifact.Read(p)
}

func (sandwich *Sandwich) Close() error {
	return nil
}

func (sandwich *Sandwich) Reset() error {
	sandwich.output = 0
	sandwich.artifact.Clear("output")

	return nil
}
