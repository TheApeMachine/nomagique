package geometry

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
Velocity tracks mean velocity between consecutive observations.
*/
type Velocity[T ~float64] struct {
	prev   float64
	ready  bool
	output core.Scalar[T]
}

/*
NewVelocity returns a velocity stage ready from its first observation.
*/
func NewVelocity[T ~float64]() *Velocity[T] {
	return &Velocity[T]{}
}

/*
Observe ingests a mean sample and returns its velocity versus the previous mean.
*/
func (velocity *Velocity[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return velocity.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return velocity.output
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	velocity.output = core.Scalar[T](T(velocity.observe(float64(sample))))

	return velocity.output
}

/*
ObserveSamples writes one velocity per mean into out.
*/
func (velocity *Velocity[T]) ObserveSamples(means []float64, out []float64) {
	for index, mean := range means {
		out[index] = velocity.observe(mean)
	}
}

/*
Reset clears derived state.
*/
func (velocity *Velocity[T]) Reset() error {
	velocity.prev = 0
	velocity.ready = false
	velocity.output = core.Scalar[T](0)

	return nil
}

func (velocity *Velocity[T]) observe(mean float64) float64 {
	if !velocity.ready {
		velocity.prev = mean
		velocity.ready = true

		return 0
	}

	derived := mean - velocity.prev
	velocity.prev = mean

	return derived
}

/*
Coupling measures directional alignment of two growth samples in [-1, +1].
*/
type Coupling[T ~float64] struct {
	output core.Scalar[T]
}

/*
NewCoupling returns a coupling stage.
*/
func NewCoupling[T ~float64]() *Coupling[T] {
	return &Coupling[T]{}
}

/*
Observe ingests left and right growth values and returns coupling strength.
*/
func (coupling *Coupling[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return coupling.output
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok {
		return coupling.output
	}

	if len(scalars) < 2 {
		return coupling.output
	}

	leftGrowth, rightGrowth, err := parseGrowthPair(scalars[0], scalars[1:])

	if err != nil {
		return coupling.output
	}

	coupling.output = core.Scalar[T](T(coupling.align(leftGrowth, rightGrowth)))

	return coupling.output
}

/*
Reset clears derived output.
*/
func (coupling *Coupling[T]) Reset() error {
	coupling.output = core.Scalar[T](0)

	return nil
}

func (coupling *Coupling[T]) align(leftGrowth, rightGrowth float64) float64 {
	absLeft := math.Abs(leftGrowth)
	absRight := math.Abs(rightGrowth)
	geometricMean := math.Sqrt(absLeft * absRight)

	if geometricMean == 0 {
		return 0
	}

	relativeFloor := (absLeft * absRight) / (absLeft + absRight + math.SmallestNonzeroFloat64)

	if geometricMean*geometricMean < relativeFloor {
		return 0
	}

	return (leftGrowth * rightGrowth) / (geometricMean * geometricMean)
}
