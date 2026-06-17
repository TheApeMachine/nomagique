package algorithm

import (
	"math"
	"time"
)

/*
EvidenceEntry stores one flagged price level.
*/
type EvidenceEntry struct {
	Expires  time.Time
	Churn    float64
	Strength float64
}

/*
EvidenceRegistry tracks near-touch toxic flags keyed by quantized price.
*/
type EvidenceRegistry struct {
	entries map[int64]EvidenceEntry
}

func NewEvidenceRegistry() *EvidenceRegistry {
	return &EvidenceRegistry{
		entries: make(map[int64]EvidenceEntry),
	}
}

func (registry *EvidenceRegistry) Flag(
	key int64,
	churnRatio float64,
	evidence float64,
	expires time.Time,
) {
	entry := registry.entries[key]
	entry.Expires = expires

	if churnRatio > 0 {
		entry.Churn = churnRatio
	}

	if evidence > 0 {
		entry.Strength = evidence
	}

	registry.entries[key] = entry
}

func (registry *EvidenceRegistry) ActiveExpiry(
	keys []int64,
	at time.Time,
) (time.Time, float64, float64, bool) {
	for _, key := range keys {
		entry, ok := registry.entries[key]

		if !ok {
			continue
		}

		if at.After(entry.Expires) {
			delete(registry.entries, key)

			continue
		}

		strength := math.Max(entry.Churn, entry.Strength)

		return entry.Expires, entry.Churn, strength, true
	}

	return time.Time{}, 0, 0, false
}

func (registry *EvidenceRegistry) Clone() *EvidenceRegistry {
	if registry == nil {
		return NewEvidenceRegistry()
	}

	next := NewEvidenceRegistry()

	for key, entry := range registry.entries {
		next.entries[key] = entry
	}

	return next
}

func (registry *EvidenceRegistry) NearTouchStrength(
	mid float64,
	proximityPct float64,
	at time.Time,
	priceFromKey func(int64) float64,
) (near bool, strength float64) {
	for key, entry := range registry.entries {
		if at.After(entry.Expires) {
			delete(registry.entries, key)

			continue
		}

		price := priceFromKey(key)

		if mid > 0 && proximityPct > 0 &&
			math.Abs(price-mid)/mid <= proximityPct {
			near = true
			strength = math.Max(strength, math.Max(entry.Churn, entry.Strength))
		}
	}

	return near, strength
}
