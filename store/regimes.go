package store

/*
Regime is one named market-state category on the grid. A category is a
confluence of cross-signal metrics (not an argmax of one signal). Key is the
region index Emit and the trie still address; Category is the regime name.
*/
type Regime struct {
	Key      uint8
	Category string
	Kind     string // action | oracle
	Phase    string // enter | exit | hold | volume | day | spread | book | board | contagion | capacity | peak
	Side     string // detector | support | opposition | veto | approve
	Metrics  []Metric
}

// Metric is one Map()-keyed score pinned into a regime category.
type Metric struct {
	Source string
	Label  string
}

func (regime Regime) Name() string {
	return regime.Category
}

/*
Regimes is the catalog of confluence categories. Desk oracle pairs
(volume / day / spread / book / peak) are set from Observe bools. Board /
contagion / capacity stay metric-pinned.
*/
type Regimes []Regime

/*
Catalog — regime categories and the metrics that compose them.

Action categories are story-phase regimes (open / stay / leave).
Oracle categories are tradability / risk regimes.
*/
var Catalog = Regimes{
	// --- Action: Enter (open) ---
	{
		Key: 0, Category: "ignition", Kind: "action", Phase: "enter", Side: "detector",
		Metrics: []Metric{
			{"cvd", "drive"},
			{"cvd", "net_fraction"},
			{"cvd", "value"},
			{"pumpdump", "compression"},
			{"pumpdump", "precursor"},
			{"pumpdump", "value"},
			{"pumpdump", "strength"},
		},
	},
	{
		Key: 1, Category: "cohort_lift", Kind: "action", Phase: "enter", Side: "support",
		Metrics: []Metric{
			{"cvd", "balance"},
			{"correlation", "herd_score"},
			{"correlation", "strength"},
			{"correlation", "correlation"},
			{"correlation", "energy"},
			{"leadlag", "inefficient_score"},
			{"leadlag", "sync_score"},
			{"leadlag", "strength"},
			{"toxicity", "support_score"},
		},
	},
	{
		Key: 2, Category: "adverse_weight", Kind: "action", Phase: "enter", Side: "opposition",
		Metrics: []Metric{
			{"cvd", "starvation"},
			{"pumpdump", "rvol"},
			{"depthflow", "loaded_score"},
			{"morphology", "bid_notional_lean"},
		},
	},

	// --- Action: Exit (leave) ---
	{
		Key: 3, Category: "absorption_fade", Kind: "action", Phase: "exit", Side: "detector",
		Metrics: []Metric{
			{"cvd", "absorption"},
			{"cvd", "net"},
			{"pumpdump", "exhaustion"},
		},
	},
	{
		Key: 4, Category: "vacuum_bluff", Kind: "action", Phase: "exit", Side: "support",
		Metrics: []Metric{
			{"depthflow", "thin_score"},
			{"depthflow", "spoof_score"},
			{"toxicity", "bluff_score"},
			{"toxicity", "vacuum_score"},
			{"toxicity", "value"},
			{"toxicity", "strength"},
		},
	},
	{
		Key: 5, Category: "trend_persistence", Kind: "action", Phase: "exit", Side: "opposition",
		Metrics: []Metric{
			{"depthflow", "loaded_score"},
			{"morphology", "ask_vacuum"},
			{"pumpdump", "trend"},
		},
	},

	// --- Action: Hold (stay) ---
	{
		Key: 6, Category: "excitation", Kind: "action", Phase: "hold", Side: "detector",
		Metrics: []Metric{
			{"hawkes", "spectral_radius"},
			{"hawkes", "branching_asymmetry"},
			{"hawkes", "total_buy_descendants"},
			{"hawkes", "total_sell_descendants"},
			{"hawkes", "immediate_buy_offspring"},
			{"hawkes", "immediate_sell_offspring"},
			{"pumpdump", "ignition"},
			{"depthflow", "hypothesis_separation"},
		},
	},
	{
		Key: 7, Category: "stable_breadth", Kind: "action", Phase: "hold", Side: "support",
		Metrics: []Metric{
			{"depthflow", "loaded_score"},
			{"depthflow", "neutral_score"},
			{"depthflow", "value"},
			{"morphology", "shape_stable"},
			{"morphology", "value"},
			{"morphology", "strength"},
			{"toxicity", "support_score"},
			{"sentiment", "surge_score"},
			{"sentiment", "breadth"},
			{"hawkes", "buy_arrival_rate"},
		},
	},
	{
		Key: 8, Category: "scarcity_exhaust", Kind: "action", Phase: "hold", Side: "opposition",
		Metrics: []Metric{
			{"pumpdump", "exhaustion"},
			{"liquidity", "scarcity_score"},
			{"depthflow", "thin_score"},
			{"sentiment", "slump_score"},
			{"hawkes", "sell_arrival_rate"},
		},
	},

	// --- Oracle: Volume Quote ---
	{Key: 9, Category: "quiet_tape", Kind: "oracle", Phase: "volume", Side: "veto"},
	{Key: 10, Category: "liquid_quote", Kind: "oracle", Phase: "volume", Side: "approve"},

	// --- Oracle: Day Move ---
	{Key: 11, Category: "flat_session", Kind: "oracle", Phase: "day", Side: "veto"},
	{Key: 12, Category: "session_move", Kind: "oracle", Phase: "day", Side: "approve"},

	// --- Oracle: Spread ---
	{Key: 13, Category: "blown_spread", Kind: "oracle", Phase: "spread", Side: "veto"},
	{Key: 14, Category: "tight_touch", Kind: "oracle", Phase: "spread", Side: "approve"},

	// --- Oracle: Book Integrity ---
	{Key: 15, Category: "brittle_book", Kind: "oracle", Phase: "book", Side: "veto"},
	{Key: 16, Category: "firm_book", Kind: "oracle", Phase: "book", Side: "approve"},

	// --- Oracle: Board Confidence ---
	{
		Key: 17, Category: "board_stress", Kind: "oracle", Phase: "board", Side: "veto",
		Metrics: []Metric{
			{"correlation", "stress_score"},
			{"sentiment", "slump_score"},
			{"pumpdump", "exhaustion"},
		},
	},
	{
		Key: 18, Category: "board_conviction", Kind: "oracle", Phase: "board", Side: "approve",
		Metrics: []Metric{
			{"correlation", "peak_score"},
			{"correlation", "herd_score"},
			{"pumpdump", "compression"},
			{"hawkes", "spectral_radius"},
			{"hawkes", "maturity"},
			{"sentiment", "surge_score"},
			{"sentiment", "breadth"},
			{"sentiment", "value"},
			{"sentiment", "strength"},
			{"depthflow", "thin_score"},
			{"toxicity", "vacuum_score"},
		},
	},

	// --- Oracle: Contagion ---
	{
		Key: 19, Category: "contagion_break", Kind: "oracle", Phase: "contagion", Side: "veto",
		Metrics: []Metric{
			{"leadlag", "decoupled_score"},
			{"leadlag", "stall_score"},
			{"correlation", "stress_score"},
			{"correlation", "noise_score"},
			{"sentiment", "divergent_score"},
		},
	},
	{
		Key: 20, Category: "contagion_sync", Kind: "oracle", Phase: "contagion", Side: "approve",
		Metrics: []Metric{
			{"leadlag", "sync_score"},
			{"correlation", "herd_score"},
			{"correlation", "alpha_score"},
		},
	},

	// --- Oracle: Capacity ---
	{
		Key: 21, Category: "capacity_tight", Kind: "oracle", Phase: "capacity", Side: "veto",
		Metrics: []Metric{
			{"liquidity", "scarcity_score"},
			{"toxicity", "vacuum_score"},
			{"depthflow", "thin_score"},
		},
	},
	{
		Key: 22, Category: "capacity_room", Kind: "oracle", Phase: "capacity", Side: "approve",
		Metrics: []Metric{
			{"liquidity", "depth_score"},
			{"liquidity", "median_score"},
			{"toxicity", "support_score"},
		},
	},

	// --- Oracle: 24h Peak ---
	{Key: 23, Category: "range_peak", Kind: "oracle", Phase: "peak", Side: "veto"},
	{Key: 24, Category: "range_room", Kind: "oracle", Phase: "peak", Side: "approve"},
}

// Pins builds source → label → region keys for Observe.
func (regimes Regimes) Pins() map[string]map[string][]uint8 {
	out := make(map[string]map[string][]uint8)

	for _, regime := range regimes {
		for _, metric := range regime.Metrics {
			if metric.Source == "" || metric.Label == "" {
				continue
			}

			byLabel := out[metric.Source]
			if byLabel == nil {
				byLabel = make(map[string][]uint8)
				out[metric.Source] = byLabel
			}

			byLabel[metric.Label] = append(byLabel[metric.Label], regime.Key)
		}
	}

	return out
}

// ByKey returns the regime for a region key, or a zero Regime.
func (regimes Regimes) ByKey(key uint8) Regime {
	for _, regime := range regimes {
		if regime.Key == key {
			return regime
		}
	}

	return Regime{}
}
