package geometry

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Velocity tracks mean velocity between consecutive observations.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Velocity struct {
	artifact *datura.Artifact
	prev     float64
	ready    bool
}

/*
NewVelocity returns a velocity stage wired from config attributes on the artifact.
*/
func NewVelocity(artifact *datura.Artifact) *Velocity {
	artifact.Inspect("geometry", "velocity", "NewVelocity()")

	return &Velocity{
		artifact: artifact,
	}
}

func (velocity *Velocity) Write(payload []byte) (int, error) {
	velocity.artifact.WithPayload(payload)
	return len(payload), nil
}

func (velocity *Velocity) Read(payload []byte) (int, error) {
	state := datura.Acquire("velocity-state", datura.APPJSON)
	state.Inspect("geometry", "velocity", "Read()", "p")

	if _, err := state.Write(velocity.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if sample == 0 && !velocity.ready {
		value := datura.Peek[float64](velocity.artifact, "output", "value")

		if velocity.ready {
			state.MergeOutput("value", value)
			state.Merge("root", "output")
			state.Merge("inputs", []string{"value"})
		}

		return state.Read(payload)
	}

	derived := velocity.observe(sample)
	velocity.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
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
	velocity.artifact.WithAttributes(datura.Map[any]{})

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
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Coupling struct {
	artifact *datura.Artifact
}

/*
NewCoupling returns a coupling stage wired from config attributes on the artifact.
*/
func NewCoupling(artifact *datura.Artifact) *Coupling {
	artifact.Inspect("geometry", "coupling", "NewCoupling()")

	return &Coupling{
		artifact: artifact,
	}
}

func (coupling *Coupling) Write(payload []byte) (int, error) {
	coupling.artifact.WithPayload(payload)
	return len(payload), nil
}

func (coupling *Coupling) Read(payload []byte) (int, error) {
	state := datura.Acquire("coupling-state", datura.APPJSON)
	state.Inspect("geometry", "coupling", "Read()", "p")

	if _, err := state.Write(coupling.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	values := datura.Peek[[]float64](state, "batch")

	if len(values) == 0 {
		left := datura.Peek[float64](state, "sample")
		right := datura.Peek[float64](state, "paired")

		if left == 0 && right == 0 {
			features := datura.Peek[[]float64](state, "features")

			if len(features) >= 2 {
				left = features[0]
				right = features[1]
			}
		}

		if left == 0 && right == 0 {
			value := datura.Peek[float64](coupling.artifact, "output", "value")

			if value != 0 {
				state.MergeOutput("value", value)
				state.Merge("root", "output")
				state.Merge("inputs", []string{"value"})
			}

			return state.Read(payload)
		}

		if right == 0 {
			return state.Read(payload)
		}

		values = []float64{left, right}
	}

	if len(values) < 2 {
		return state.Read(payload)
	}

	leftGrowth, rightGrowth, err := parseGrowthPair(values[0], values[1:])

	if err != nil {
		return state.Read(payload)
	}

	derived := coupling.align(leftGrowth, rightGrowth)
	coupling.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (coupling *Coupling) Close() error {
	return nil
}

func (coupling *Coupling) Reset() error {
	coupling.artifact.WithAttributes(datura.Map[any]{})

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
