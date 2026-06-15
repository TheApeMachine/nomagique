package geometry

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Velocity tracks mean velocity between consecutive observations.
*/
type Velocity struct {
	artifact *datura.Artifact
	prev     float64
	ready    bool
	output   float64
}

/*
NewVelocity returns a velocity stage ready from its first observation.
*/
func NewVelocity() *Velocity {
	return &Velocity{
		artifact: datura.Acquire("velocity", datura.Artifact_Type_json),
	}
}

func (velocity *Velocity) Write(p []byte) (int, error) {
	return velocity.artifact.Write(p)
}

func (velocity *Velocity) Read(p []byte) (int, error) {
	sample, ok := boundaryFloat64(velocity.artifact)

	if ok {
		velocity.output = velocity.observe(sample)
		putFloat64Payload(&velocity.artifact, "phase", velocity.output)
	}

	return velocity.artifact.Read(p)
}

func (velocity *Velocity) Close() error {
	return nil
}

/*
ObserveSamples writes one velocity per mean into out.
*/
func (velocity *Velocity) ObserveSamples(means []float64, out []float64) {
	for index, mean := range means {
		out[index] = velocity.observe(mean)
	}
}

func (velocity *Velocity) Reset() error {
	velocity.prev = 0
	velocity.ready = false
	velocity.output = 0

	return nil
}

func (velocity *Velocity) observe(mean float64) float64 {
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
type Coupling struct {
	artifact *datura.Artifact
	output   float64
}

/*
NewCoupling returns a coupling stage.
*/
func NewCoupling() *Coupling {
	return &Coupling{
		artifact: datura.Acquire("coupling", datura.Artifact_Type_json),
	}
}

func (coupling *Coupling) Write(p []byte) (int, error) {
	return coupling.artifact.Write(p)
}

func (coupling *Coupling) Read(p []byte) (int, error) {
	values := float64Batch(coupling.artifact)

	if len(values) >= 2 {
		leftGrowth, rightGrowth, err := parseGrowthPair(values[0], values[1:])

		if err == nil {
			coupling.output = coupling.align(leftGrowth, rightGrowth)
			putFloat64Payload(&coupling.artifact, "phase", coupling.output)
		}
	}

	return coupling.artifact.Read(p)
}

func (coupling *Coupling) Close() error {
	return nil
}

func (coupling *Coupling) Reset() error {
	coupling.output = 0

	return nil
}

func (coupling *Coupling) align(leftGrowth, rightGrowth float64) float64 {
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
