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

Read expects a 16-byte payload: sample value as big-endian float64, then event
time as Unix nanoseconds encoded in float64.
*/
type TimeElastic struct {
	artifact *datura.Artifact
	memory   *TimeElasticMemory
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
		relative, updateErr := timeElastic.memory.Update(eventAt, sample)

		if updateErr == nil {
			assignScalarPayload(&timeElastic.artifact, "time-elastic", relative)
		}

		return timeElastic.artifact.Read(p)
	}

	if err == nil && len(payload) >= 8 {
		assignScalarPayload(&timeElastic.artifact, "time-elastic", 0)
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
	return timeElastic.readSampleViaArtifact(sample, eventAt)
}

/*
Reset clears derived baseline state.
*/
func (timeElastic *TimeElastic) Reset() error {
	timeElastic.memory.Reset()

	return nil
}

func (timeElastic *TimeElastic) readSampleViaArtifact(sample float64, eventAt time.Time) float64 {
	inbound := datura.Acquire("time-elastic-in", datura.Artifact_Type_json)
	payload := make([]byte, 16)
	binary.BigEndian.PutUint64(payload[:8], math.Float64bits(sample))
	binary.BigEndian.PutUint64(payload[8:16], math.Float64bits(float64(eventAt.UnixNano())))
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = timeElastic.Write(buf)
	outBuf := make([]byte, 4096)
	_, _ = timeElastic.Read(outBuf)
	outbound := datura.Acquire("stage-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

/*
Value returns the last derived scalar without re-processing the stage.
*/
func (timeElastic *TimeElastic) Value() float64 {
	return valueFromArtifact(timeElastic.artifact)
}
