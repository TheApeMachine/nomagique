package manifold

/*
Config holds the predictive-coding resonance hyperparameters for the GPU solver.

Every value mirrors learning.ResonanceConfig so the Metal manifold settles the
same energy landscape the gonum reference settles. The GPU path is float32, so
parity is behavioral (tolerance-checked) rather than bit-exact.
*/
type Config struct {
	MaxInferenceSteps  int
	MinInferenceSteps  int
	LrState            float64
	EarlyStopTol       float64
	EarlyStopPatience  int
	MonotoneStateSteps bool
	LineSearchHalvings int

	LrGenerative  float64
	LrTemporal    float64
	LrRecognition float64

	TemporalWeight float64
	TopDownInitMix float64

	UsePrecision  bool
	PrecisionBeta float64
	PrecisionMin  float64
	PrecisionMax  float64
	PrecisionEps  float64

	LatentDecay float64
	Sparsity    float64
	WeightDecay float64
	GradClip    float64
	StateClip   float64
}

/*
AdaptiveConfig derives every hyperparameter dynamically from the system-wide
learning pace (alpha) and the physical depth of the network, identically to
learning.AdaptiveResonanceConfig.
*/
func AdaptiveConfig(alpha float64, arch []int) Config {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.010
	}

	depth := len(arch)

	return Config{
		MaxInferenceSteps:  depth * 8,
		MinInferenceSteps:  depth * 2,
		LrState:            alpha * 10.0,
		EarlyStopTol:       1e-5,
		EarlyStopPatience:  3,
		MonotoneStateSteps: true,
		LineSearchHalvings: 3,

		LrGenerative:  alpha * 1.0,
		LrTemporal:    alpha * 2.0,
		LrRecognition: alpha * 0.6,

		TemporalWeight: 0.45,
		TopDownInitMix: 0.55,

		UsePrecision:  true,
		PrecisionBeta: alpha,
		PrecisionMin:  0.10,
		PrecisionMax:  5.0,
		PrecisionEps:  1e-4,

		LatentDecay: alpha * 1e-1,
		Sparsity:    alpha * 1e-2,
		WeightDecay: alpha * 1e-3,
		GradClip:    1.0,
		StateClip:   3.0,
	}
}
