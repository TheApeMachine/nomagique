package adaptive

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/theapemachine/datura"
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
		epsilon = defaultTimeElasticEpsilon()
	}

	return &TimeElasticMemory{
		halflife: halflife,
		epsilon:  epsilon,
	}
}

func (memory *TimeElasticMemory) Initialized() bool {
	return memory.initialized
}

func defaultTimeElasticEpsilon() float64 {
	return math.Sqrt(math.Nextafter(1, 2) - 1)
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

Read expects a 16-byte payload: sample value as big-endian float64, then event
time as Unix nanoseconds encoded in float64.
*/
type TimeElastic struct {
	artifact *datura.Artifact
	memory   *TimeElasticMemory
	value    float64
}

/*
NewTimeElastic returns a time-elastic baseline stage.
*/
func NewTimeElastic(halflife time.Duration, epsilon float64) *TimeElastic {
	return &TimeElastic{
		artifact: datura.Acquire("time_elastic", datura.Artifact_Type_json),
		memory:   NewTimeElasticMemory(halflife, epsilon),
	}
}

func (timeElastic *TimeElastic) Write(p []byte) (int, error) {
	return timeElastic.artifact.Write(p)
}

func (timeElastic *TimeElastic) Read(p []byte) (int, error) {
	payload, err := timeElastic.artifact.Payload()

	if err == nil && len(payload) >= 16 {
		sample := math.Float64frombits(binary.BigEndian.Uint64(payload[:8]))
		eventAt := time.Unix(0, int64(math.Float64frombits(binary.BigEndian.Uint64(payload[8:16]))))
		derived := timeElastic.observe(sample, eventAt)
		assignScalarPayload(&timeElastic.artifact, "time-elastic", derived)

		return timeElastic.artifact.Read(p)
	}

	if err == nil && len(payload) >= 8 {
		assignScalarPayload(&timeElastic.artifact, "time-elastic", timeElastic.value)
	}

	return timeElastic.artifact.Read(p)
}

func (timeElastic *TimeElastic) Close() error {
	return nil
}

/*
ObserveSample ingests sample and event time through the time-elastic kernel.
*/
func (timeElastic *TimeElastic) ObserveSample(sample float64, eventAt time.Time) float64 {
	derived := timeElastic.observe(sample, eventAt)
	assignScalarPayload(&timeElastic.artifact, "time-elastic", derived)

	return derived
}

/*
Reset clears derived baseline state.
*/
func (timeElastic *TimeElastic) Reset() error {
	timeElastic.memory.Reset()
	timeElastic.value = 0
	assignScalarPayload(&timeElastic.artifact, "time-elastic", 0)

	return nil
}

func (timeElastic *TimeElastic) observe(sample float64, eventAt time.Time) float64 {
	if !finiteScalar(sample) {
		return timeElastic.value
	}

	relative, err := timeElastic.memory.Update(eventAt, sample)

	if err != nil || !finiteScalar(relative) {
		return timeElastic.value
	}

	timeElastic.value = relative

	return relative
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (timeElastic *TimeElastic) Value() float64 {
	return timeElastic.value
}
