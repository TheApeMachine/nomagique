package store

import (
	"context"
	"testing"
)

func TestGridObserveSkipsUnpinnedLabelsWithoutError(t *testing.T) {
	grid := NewGrid(context.Background())

	// pumpdump has pinned labels and we add an unpinned one
	sources := map[string]map[string]float64{
		"pumpdump": {
			"coiled_compression": 0.5,
			"unpinned_metric":    0.9,
		},
	}

	regions, err := grid.Observe("BTC/USD", sources, false)
	if err != nil {
		t.Fatalf("expected Observe to succeed despite unpinned label, got: %v", err)
	}

	if len(regions) == 0 {
		t.Fatal("expected at least one region from pinned label, got 0")
	}
}

func TestGridObserveBrightnessNonZero(t *testing.T) {
	grid := NewGrid(context.Background())

	sources := map[string]map[string]float64{
		"pumpdump": {
			"coiled_compression": 1.5,
			"rejection_tick":     0.3,
		},
		"correlation": {
			"structural_alignment": 0.8,
		},
		"depthflow": {
			"bid_ladder_build": 2.0,
		},
	}

	regions, err := grid.Observe("BTC/USD", sources, false)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	if len(regions) == 0 {
		t.Fatal("expected regions, got 0")
	}

	nonZero := 0
	for _, r := range regions {
		if r.Brightness > 0 {
			nonZero++
			t.Logf("region R%02d brightness=%.4f deformation=%.4f counts=%d",
				r.Key, r.Brightness, r.Deformation, r.Counts)
		}
	}

	if nonZero == 0 {
		t.Fatal("expected at least one region with Brightness > 0, all were 0")
	}
}

func TestGridObserveEnterSparkTop2Corroboration(t *testing.T) {
	grid := NewGrid(context.Background())

	regions, err := grid.Observe("PEP/USD", map[string]map[string]float64{
		"pumpdump": {"coiled_compression": 2.0},
		"cvd":      {"aggressor_backing": 1.0},
	}, false)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	var r00 *Region
	for _, r := range regions {
		if r.Key == 0 {
			r00 = r
			break
		}
	}

	if r00 == nil {
		t.Fatal("expected R00 from coiled + cvd corroboration")
	}

	if r00.Counts != 2 {
		t.Fatalf("expected 2 metrics on R00, got %d", r00.Counts)
	}

	if r00.Brightness <= 0 {
		t.Fatalf("expected positive R00 brightness, got %.4f", r00.Brightness)
	}
}

func TestGridObserveCoiledLightsEnterDetector(t *testing.T) {
	grid := NewGrid(context.Background())

	regions, err := grid.Observe("PEP/USD", map[string]map[string]float64{
		"pumpdump": {"coiled_compression": 1.2},
	}, false)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	bright := map[uint8]float64{}
	for _, r := range regions {
		if r.Brightness > 0 {
			bright[r.Key] = r.Brightness
		}
	}

	if bright[0] <= 0 {
		t.Fatalf("expected enter detector R00 lit, got %v", bright)
	}
	if bright[6] > 0 {
		t.Fatalf("coiled_compression must not light hold detector R06, got %v", bright)
	}
}

func TestGridObserveBoardCoiledLightsEnterConfidence(t *testing.T) {
	grid := NewGrid(context.Background())

	regions, err := grid.Observe("PEAQ/USD", map[string]map[string]float64{
		"pumpdump": {"board_coiled": 1.4},
	}, false)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	bright := map[uint8]float64{}
	for _, r := range regions {
		if r.Brightness > 0 {
			bright[r.Key] = r.Brightness
		}
	}

	if bright[9] <= 0 {
		t.Fatalf("expected enter confidence R09 lit, got %v", bright)
	}
}

func TestGridObserveAskLadderReleaseLightsExitOpposition(t *testing.T) {
	grid := NewGrid(context.Background())

	regions, err := grid.Observe("PEAQ/USD", map[string]map[string]float64{
		"depthflow": {"ask_ladder_release": 1.9},
	}, false)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	bright := map[uint8]float64{}
	for _, r := range regions {
		if r.Brightness > 0 {
			bright[r.Key] = r.Brightness
		}
	}

	if bright[5] <= 0 {
		t.Fatalf("expected exit opposition R05 lit, got %v", bright)
	}
	if bright[7] > 0 {
		t.Fatalf("ask_ladder_release must not light hold support R07, got %v", bright)
	}
}

func TestGridObserveBidLadderBuildLightsHoldSupportOnly(t *testing.T) {
	grid := NewGrid(context.Background())

	regions, err := grid.Observe("PEAQ/USD", map[string]map[string]float64{
		"depthflow": {"bid_ladder_build": 2.0},
	}, false)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	bright := map[uint8]float64{}
	for _, r := range regions {
		if r.Brightness > 0 {
			bright[r.Key] = r.Brightness
		}
	}

	if bright[7] <= 0 {
		t.Fatalf("expected hold support R07 lit, got %v", bright)
	}
	if bright[5] > 0 {
		t.Fatalf("bid_ladder_build must not light exit opposition R05, got %v", bright)
	}
}

func TestGridObserveDepthflowReleaseLightsExitSupport(t *testing.T) {
	grid := NewGrid(context.Background())

	sources := map[string]map[string]float64{
		"depthflow": {
			"bid_ladder_release": 2.63,
		},
	}

	regions, err := grid.Observe("PEAQ/USD", sources, false)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}

	bright := map[uint8]float64{}
	for _, r := range regions {
		if r.Brightness > 0 {
			bright[r.Key] = r.Brightness
		}
	}

	if bright[4] <= 0 {
		t.Fatalf("expected exit support R04 lit, got brightness map %v", bright)
	}
	for _, key := range []uint8{2, 8} {
		if bright[key] > 0 {
			t.Fatalf("bid_ladder_release must not light R%02d, got %v", key, bright)
		}
	}
}

func TestGridObserveBrightnessStaysNonZeroOnRepeat(t *testing.T) {
	grid := NewGrid(context.Background())

	sources := map[string]map[string]float64{
		"pumpdump": {
			"coiled_compression": 0.8,
		},
	}

	// Observe the same signal 5 times — brightness must stay non-zero
	// because evidence is present, even though the value doesn't change.
	for tick := 0; tick < 5; tick++ {
		regions, err := grid.Observe("BTC/USD", sources, false)
		if err != nil {
			t.Fatalf("tick %d: Observe failed: %v", tick, err)
		}

		found := false
		for _, r := range regions {
			if r.Key == 0 && r.Brightness > 0 {
				found = true
			}
		}

		if !found {
			t.Fatalf("tick %d: R00 brightness was 0 despite coiled_compression=0.8", tick)
		}
	}
}
