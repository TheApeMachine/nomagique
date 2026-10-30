package store

import (
	"context"
	"testing"
)

func TestGridObserveEnterOppositionUsesLevel(t *testing.T) {
	grid := NewGrid(context.Background())

	// Warm priors so SNM surprise fades while level stays at chronic starved=1.
	var r02Enter float64
	for tick := 0; tick < 8; tick++ {
		regions, err := grid.Observe("DCR/USD", map[string]map[string]float64{
			"pumpdump": {"starved_tape": 1.0},
		}, true)
		if err != nil {
			t.Fatalf("tick %d: Observe failed: %v", tick, err)
		}
		for _, r := range regions {
			if r.Key == 2 {
				r02Enter = r.Brightness
			}
		}
	}

	if r02Enter <= 0.5 {
		t.Fatalf("expected enter-phase R02 to reflect chronic starved level, got %.4f", r02Enter)
	}

	regions, err := grid.Observe("DCR/USD", map[string]map[string]float64{
		"pumpdump": {"starved_tape": 1.0},
	}, false)
	if err != nil {
		t.Fatalf("hold Observe failed: %v", err)
	}

	var r02Hold float64
	for _, r := range regions {
		if r.Key == 2 {
			r02Hold = r.Brightness
		}
	}

	if r02Hold > r02Enter+1e-9 {
		t.Fatalf("hold-phase R02 should not exceed enter starved level; hold=%.4f enter=%.4f", r02Hold, r02Enter)
	}
}

func TestGridSymbolMaturity(t *testing.T) {
	grid := NewGrid(context.Background())

	if m := grid.SymbolMaturity("PEP/USD"); m != 0 {
		t.Fatalf("expected 0 maturity before observations, got %v", m)
	}

	for range 4 {
		if _, err := grid.Observe("PEP/USD", map[string]map[string]float64{
			"pumpdump": {"coiled_compression": 0.5},
		}, false); err != nil {
			t.Fatal(err)
		}
	}

	if m := grid.SymbolMaturity("PEP/USD"); m <= 0 {
		t.Fatalf("expected positive maturity after samples, got %v", m)
	}
}
