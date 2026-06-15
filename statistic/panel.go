package statistic

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/theapemachine/datura"
)

/*
Panel is a live registry of keyed numeric samples.
*/
type Panel struct {
	artifact *datura.Artifact
	values   sync.Map
	output   float64
}

/*
NewPanel returns a keyed sample registry for cross-section pipelines.
*/
func NewPanel() *Panel {
	return &Panel{
		artifact: datura.Acquire("panel", datura.Artifact_Type_json),
	}
}

func (panel *Panel) Write(p []byte) (int, error) {
	return panel.artifact.Write(p)
}

func (panel *Panel) Read(p []byte) (int, error) {
	payload, err := panel.artifact.Payload()

	if err == nil && len(payload) >= 16 && len(payload)%8 == 0 {
		count := len(payload) / 8
		values := make([]float64, count)

		for index := range count {
			offset := index * 8
			values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
		}

		if len(values) >= 2 {
			memberKey := values[0]
			sampleValue := values[1]
			panel.values.Store(memberKey, sampleValue)
			panel.output = sampleValue
		}
	}

	if err == nil && len(payload) == 8 {
		panel.output = math.Float64frombits(binary.BigEndian.Uint64(payload))
	}

	panel.publishPeerBatch()

	peerPayload, payloadErr := panel.artifact.Payload()

	if payloadErr != nil || len(peerPayload) == 0 {
		putFloat64Payload(&panel.artifact, "panel", panel.output)
	}

	return panel.artifact.Read(p)
}

func (panel *Panel) publishPeerBatch() {
	peerPayload := make([]byte, 0)

	panel.values.Range(func(key, value any) bool {
		memberKey, keyOK := key.(float64)
		sample, valueOK := value.(float64)

		if !keyOK || !valueOK {
			return true
		}

		memberBytes := make([]byte, 8)
		sampleBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(memberBytes, math.Float64bits(memberKey))
		binary.BigEndian.PutUint64(sampleBytes, math.Float64bits(sample))
		peerPayload = append(peerPayload, memberBytes...)
		peerPayload = append(peerPayload, sampleBytes...)

		return true
	})

	if len(peerPayload) == 0 {
		return
	}

	_ = panel.artifact.SetPayload(peerPayload)
}

func (panel *Panel) Close() error {
	return nil
}

/*
Value returns the last stored panel sample without re-processing.
*/
func (panel *Panel) Value() float64 {
	return panel.output
}

func (panel *Panel) Reset() error {
	panel.values = sync.Map{}
	panel.output = 0

	return nil
}

func (panel *Panel) peerSamples(excludedKey float64) []float64 {
	peerSamples := make([]float64, 0)

	panel.values.Range(func(key, value any) bool {
		memberKey, keyOK := key.(float64)
		sample, valueOK := value.(float64)

		if !keyOK || !valueOK || memberKey == excludedKey {
			return true
		}

		peerSamples = append(peerSamples, sample)

		return true
	})

	return peerSamples
}
