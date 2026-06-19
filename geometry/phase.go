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
}

/*
NewVelocity returns a velocity stage ready from its first observation.
*/
func NewVelocity() *Velocity {
	return &Velocity{
		artifact: datura.Acquire("velocity", datura.APPJSON).RetainStageAttributes(),
	}
}

func (velocity *Velocity) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](velocity.artifact, "output") == nil

	velocity.artifact.Clear("sample")

	n, err := velocity.artifact.Write(p)

	if bootstrap {
		velocity.artifact.Clear("output")
	}

	return n, err
}

func (velocity *Velocity) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](velocity.artifact, "sample")

	if sample == 0 && !velocity.ready {
		return velocity.artifact.Read(p)
	}

	derived := velocity.observe(sample)
	velocity.artifact.Poke(datura.Map[float64]{"value": derived}, "output")

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
	velocity.artifact.Clear("output")

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
}

/*
NewCoupling returns a coupling stage.
*/
func NewCoupling() *Coupling {
	return &Coupling{
		artifact: datura.Acquire("coupling", datura.APPJSON).RetainStageAttributes(),
	}
}

func (coupling *Coupling) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](coupling.artifact, "output") == nil

	coupling.artifact.Clear("sample")
	coupling.artifact.Clear("paired")
	coupling.artifact.Clear("batch")

	n, err := coupling.artifact.Write(p)

	if bootstrap {
		coupling.artifact.Clear("output")
	}

	return n, err
}

func (coupling *Coupling) Read(p []byte) (int, error) {
	values := datura.Peek[[]float64](coupling.artifact, "batch")

	if len(values) == 0 {
		left := datura.Peek[float64](coupling.artifact, "sample")
		right := datura.Peek[float64](coupling.artifact, "paired")

		if left == 0 && right == 0 {
			return coupling.artifact.Read(p)
		}

		if right == 0 {
			return coupling.artifact.Read(p)
		}

		values = []float64{left, right}
	}

	if len(values) < 2 {
		return coupling.artifact.Read(p)
	}

	leftGrowth, rightGrowth, err := parseGrowthPair(values[0], values[1:])

	if err != nil {
		return coupling.artifact.Read(p)
	}

	coupling.artifact.Poke(
		datura.Map[float64]{"value": coupling.align(leftGrowth, rightGrowth)},
		"output",
	)

	return coupling.artifact.Read(p)
}

func (coupling *Coupling) Close() error {
	return nil
}

func (coupling *Coupling) Reset() error {
	coupling.artifact.Clear("output")

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
