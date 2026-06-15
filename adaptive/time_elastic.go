package adaptive

import (
	"fmt"
	"math"
	"time"

	"github.com/theapemachine/nomagique/core"
)

/*
TimeElasticMemory tracks a time-decayed baseline and returns sample/baseline
before each update. Alpha uses halflife with tau = halflife / ln(2).
*/
type TimeElasticMemory struct {
	halflife    time.Duration
	epsilon     float64
	vSlow       float64
	lastAt      time.Time
	initialized bool
}

func NewTimeElasticMemory(halflife time.Duration, epsilon float64) *TimeElasticMemory {
	if epsilon <= 0 {
		epsilon = 1e-6
	}

	return &TimeElasticMemory{
		halflife: halflife,
		epsilon:  epsilon,
	}
}

func (memory *TimeElasticMemory) Initialized() bool {
	return memory.initialized
}

func (memory *TimeElasticMemory) Update(eventAt time.Time, currentSample float64) (float64, error) {
	if currentSample < 0 {
		return 0, fmt.Errorf("adaptive: TimeElasticMemory negative sample")
	}

	if memory.halflife <= 0 {
		return 0, fmt.Errorf("adaptive: TimeElasticMemory halflife must be positive")
	}

	if eventAt.IsZero() {
		return 0, fmt.Errorf("adaptive: TimeElasticMemory timestamp is required")
	}

	if !memory.initialized || memory.lastAt.IsZero() {
		memory.vSlow = currentSample
		memory.lastAt = eventAt
		memory.initialized = true

		return 1.0, nil
	}

	delta := eventAt.Sub(memory.lastAt)

	if delta < 0 {
		delta = 0
	}

	memory.lastAt = eventAt

	tau := float64(memory.halflife) / math.Ln2

	var alpha float64

	if tau > 0 && delta > 0 {
		alpha = 1.0 - math.Exp(-float64(delta)/tau)
	}

	if delta > 0 && tau <= 0 {
		alpha = 1.0
	}

	relative := currentSample / (memory.vSlow + memory.epsilon)

	memory.vSlow = (1.0-alpha)*memory.vSlow + alpha*currentSample

	return relative, nil
}

/*
Reset clears derived baseline state.
*/
func (memory *TimeElasticMemory) Reset() {
	memory.vSlow = 0
	memory.lastAt = time.Time{}
	memory.initialized = false
}

/*
TimeElastic tracks a time-decayed baseline and returns sample/baseline ratios.

Observe expects two scalar inputs: sample value, then event time as Unix
nanoseconds encoded in float64. Fewer than two scalars returns the prior output.
*/
type TimeElastic[T ~float64] struct {
	memory *TimeElasticMemory
	output core.Scalar[T]
}

/*
NewTimeElastic returns a time-elastic baseline stage for nomagique.Number pipelines.
*/
func NewTimeElastic[T ~float64](
	halflife time.Duration, epsilon float64,
) *TimeElastic[T] {
	return &TimeElastic[T]{
		memory: NewTimeElasticMemory(halflife, epsilon),
	}
}

/*
Observe ingests sample and event-time scalars and returns the relative baseline ratio.
*/
func (timeElastic *TimeElastic[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if len(inputs) == 0 {
		return timeElastic.output
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return timeElastic.output
	}

	if len(inputs) < 2 {
		return timeElastic.output
	}

	timestamp, timestampOK := inputs[1].(core.Scalar[T])

	if !timestampOK {
		return timeElastic.output
	}

	eventAt := time.Unix(0, int64(float64(timestamp)))

	relative, err := timeElastic.memory.Update(eventAt, float64(sample))

	if err != nil {
		return timeElastic.output
	}

	timeElastic.output = core.Scalar[T](T(relative))

	return timeElastic.output
}

/*
Reset clears derived baseline state.
*/
func (timeElastic *TimeElastic[T]) Reset() error {
	timeElastic.memory.Reset()
	timeElastic.output = core.Scalar[T](0)

	return nil
}
