package store

/*
regionMetricPins maps wire labels (envelope.Symbol) to confluence regions.

Action: Enter R00–R02, Exit R03–R05, Hold R06–R08.
Oracle: Board Confidence R09–R11, Friction/Spread R12, Contagion R13, Capacity R14.

A label may pin to more than one region when SIGNALS.md shares it across tokens.
*/
var regionMetricPins = map[string]map[string][]uint8{
	"cvd": {
		"drive":        {0},
		"net_fraction": {0},
		"balance":      {1},
		"starvation":   {2},
		"absorption":   {3},
		"net":          {3},
		"value":        {0},
	},
	"pumpdump": {
		"compression": {0, 9},
		"precursor":   {0},
		"rvol":        {2},
		"exhaustion":  {3, 8, 9},
		"ignition":    {6},
		"trend":       {5},
		"spread":      {12},
		"value":       {0},
		"strength":    {0},
	},
	"correlation": {
		"herd_score":   {1, 9, 13},
		"strength":     {1},
		"peak_score":   {9},
		"alpha_score":  {13},
		"noise_score":  {13},
		"stress_score": {9, 13},
		"correlation":  {1},
		"energy":       {1},
	},
	"leadlag": {
		"inefficient_score": {1},
		"sync_score":        {1, 13},
		"decoupled_score":   {13},
		"stall_score":       {13},
		"strength":          {1},
	},
	"depthflow": {
		"loaded_score":          {2, 5, 7},
		"thin_score":            {4, 8, 11, 14},
		"spoof_score":           {4},
		"hypothesis_separation": {6},
		"neutral_score":         {7},
		"value":                 {7},
	},
	"toxicity": {
		"support_score": {1, 7, 12, 14},
		"bluff_score":   {4},
		"vacuum_score":  {4, 11, 14},
		"value":         {4},
		"strength":      {4},
	},
	"morphology": {
		"bid_notional_lean": {2},
		"ask_vacuum":        {5},
		"shape_stable":      {7},
		"value":             {7},
		"strength":          {7},
	},
	"hawkes": {
		"spectral_radius":          {6, 10},
		"branching_asymmetry":      {6, 10},
		"total_buy_descendants":    {6},
		"total_sell_descendants":   {6},
		"buy_arrival_rate":         {7},
		"sell_arrival_rate":        {8},
		"immediate_buy_offspring":  {6},
		"immediate_sell_offspring": {6},
		"maturity":                 {10},
	},
	"sentiment": {
		"surge_score":     {7, 10},
		"breadth":         {7, 10},
		"slump_score":     {8, 10},
		"divergent_score": {13},
		"change":          {10},
		"value":           {10},
		"strength":        {10},
	},
	"liquidity": {
		"scarcity_score": {8, 12, 14},
		"median_score":   {12, 14},
		"depth_score":    {12, 14},
		"value":          {12},
		"strength":       {12},
	},
}
