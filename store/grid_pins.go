package store

/*
regionMetricPins maps wire labels to confluence regions (0..14).

Every story region R00–R14 has exactly two pinned metrics so top-2 pooling
is the arithmetic mean of two independent channels (order-statistics unbiased).
*/
var regionMetricPins = map[string]map[string][]uint8{
	"cvd": {
		"aggressor_backing": {0},
		"cvd_divergence":    {3},
	},
	"pumpdump": {
		"coiled_compression": {0},
		"starved_tape":       {2},
		"rejection_tick":     {3},
		"clock_acceleration": {6},
		"blowoff_dip":        {8},
		"board_coiled":       {9},
	},
	"correlation": {
		"structural_alignment":     {1},
		"board_dispersion_squeeze": {9},
		"structure_uncoupling":     {13},
	},
	"leadlag": {
		"led_peer_share":   {1},
		"anchor_breakdown": {13},
	},
	"depthflow": {
		"ask_absorption":     {2},
		"bid_ladder_release": {4},
		"ask_ladder_release": {5},
		"bid_ladder_build":   {7},
		"board_bid_release":  {11},
	},
	"toxicity": {
		"retreat_fraction":      {4},
		"board_withdrawal_rate": {11},
	},
	"morphology": {
		"ask_vacuum":        {5},
		"book_shape_stable": {7},
	},
	"hawkes": {
		"supercritical_cascade": {6},
		"board_cascade":         {10},
	},
	"sentiment": {
		"cohort_still_aligned": {10},
	},
	"liquidity": {
		"spread_expansion":    {8},
		"market_friction":     {12},
		"friction_drag_ratio": {12},
		"capacity_stress":     {14},
		"liquidation_impact":  {14},
	},
}
