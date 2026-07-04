package algorithm

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
)

/*
Pearl measures a direct float-only ladder-of-causation view.
It owns sampling, causal scoring, and category classification.
*/
type Pearl struct {
	sample     *PearlSample
	classifier *probability.ScoreClassifier
}

/*
PearlConfig configures the direct Pearl calculator.
*/
type PearlConfig struct {
	MinHistory      int
	History         int
	CategoryIndexes []float64
}

/*
PearlOutput contains causal rung scores and classification output.
*/
type PearlOutput struct {
	Value              float64
	Category           float64
	Confidence         float64
	ConfidenceBaseline float64
	EntryBaseline      float64
	ExitBaseline       float64
	Strength           float64
	AlphaScore         float64
	BetaScore          float64
	ShockScore         float64
	NoiseScore         float64
	Association        float64
	Intervention       float64
	InterventionScore  float64
	Uplift             float64
	UpliftScore        float64
	Contagion          float64
	Condition          float64
	Inverted           bool
	Probabilities      []float64
	Distribution       map[string]float64
}

/*
NewPearl returns a direct Pearl calculator.
*/
func NewPearl(configs ...PearlConfig) *Pearl {
	config := PearlConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &Pearl{
		sample: NewPearlSample(config),
		classifier: probability.NewScoreClassifier(
			[]string{"alphaScore", "betaScore", "shockScore", "noiseScore"},
			config.CategoryIndexes,
		),
	}
}

/*
MeasureTicker observes one ticker row and returns a causal output when ready.
*/
func (pearl *Pearl) MeasureTicker(
	input PearlTickerInput,
) (PearlOutput, bool, error) {
	sample, ready, err := pearl.sample.MeasureTicker(input)

	return pearl.measure(sample, ready, err)
}

/*
MeasureBook observes one book row and returns a causal output when ready.
*/
func (pearl *Pearl) MeasureBook(
	input PearlBookInput,
) (PearlOutput, bool, error) {
	sample, ready, err := pearl.sample.MeasureBook(input)

	return pearl.measure(sample, ready, err)
}

/*
MeasureTrade observes one trade row and returns a causal output when ready.
*/
func (pearl *Pearl) MeasureTrade(
	input PearlTradeInput,
) (PearlOutput, bool, error) {
	sample, ready, err := pearl.sample.MeasureTrade(input)

	return pearl.measure(sample, ready, err)
}

func (pearl *Pearl) measure(
	sample PearlSampleOutput,
	ready bool,
	err error,
) (PearlOutput, bool, error) {
	if err != nil {
		return PearlOutput{}, false, err
	}

	if !ready {
		return PearlOutput{}, false, nil
	}

	output, ok, err := newPearlAnalyzer(sample.Rows, sample.Row).Measure()

	if err != nil || !ok {
		return output, ok, err
	}

	result, err := pearl.classifier.Classify(map[string]float64{
		"alphaScore": output.AlphaScore,
		"betaScore":  output.BetaScore,
		"shockScore": output.ShockScore,
		"noiseScore": output.NoiseScore,
		"strength":   output.Strength,
	})

	if err != nil {
		return PearlOutput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"pearl: classification failed",
			err,
		))
	}

	output.Value = result.Value
	output.Category = result.Category
	output.Confidence = result.Confidence
	output.ConfidenceBaseline = result.ConfidenceBaseline
	output.EntryBaseline = result.EntryBaseline
	output.ExitBaseline = result.ExitBaseline
	output.Probabilities = result.Probabilities
	output.Distribution = result.Distribution

	return output, true, nil
}

/*
Outputs returns the map shape expected by signal measurement artifacts.
*/
func (output PearlOutput) Outputs() map[string]any {
	return map[string]any{
		"alphaScore":          output.AlphaScore,
		"betaScore":           output.BetaScore,
		"shockScore":          output.ShockScore,
		"noiseScore":          output.NoiseScore,
		"association":         output.Association,
		"intervention":        output.Intervention,
		"interventionScore":   output.InterventionScore,
		"uplift":              output.Uplift,
		"upliftScore":         output.UpliftScore,
		"contagion":           output.Contagion,
		"condition":           output.Condition,
		"inverted":            output.invertedValue(),
		"strength":            output.Strength,
		"value":               output.Value,
		"category":            output.Category,
		"confidence":          output.Confidence,
		"confidence_baseline": output.ConfidenceBaseline,
		"entry_baseline":      output.EntryBaseline,
		"exit_baseline":       output.ExitBaseline,
		"probabilities":       output.Probabilities,
		"distribution":        output.Distribution,
	}
}

func (output PearlOutput) invertedValue() float64 {
	if output.Inverted {
		return 1
	}

	return 0
}

type pearlAnalyzer struct {
	rows    [][]float64
	current []float64
}

func newPearlAnalyzer(rows [][]float64, current []float64) *pearlAnalyzer {
	return &pearlAnalyzer{
		rows:    rows,
		current: current,
	}
}

func (analyzer *pearlAnalyzer) Measure() (PearlOutput, bool, error) {
	if len(analyzer.rows) == 0 || len(analyzer.current) != pearlSampleNodeCount {
		return PearlOutput{}, false, nil
	}

	flow := analyzer.column(pearlNodeFlow)
	target := analyzer.column(pearlNodeTarget)
	macro := analyzer.column(pearlNodeMacro)
	liquidity := analyzer.column(pearlNodeLiquidity)
	targetScale := analyzer.scale(target)

	if targetScale <= 0 {
		return PearlOutput{}, false, nil
	}

	association := math.Abs(analyzer.correlation(flow, target))
	macroAssociation := math.Abs(analyzer.correlation(macro, target))
	liquidityAssociation := math.Abs(analyzer.correlation(liquidity, target))
	intervention := analyzer.slope(flow, target) * analyzer.scale(flow)
	interventionScore := math.Abs(intervention) / targetScale
	uplift := analyzer.uplift(flow, target)
	upliftScore := math.Abs(uplift) / targetScale
	contagion := math.Max(macroAssociation, liquidityAssociation)
	condition := analyzer.condition(flow, liquidity)
	inverted := analyzer.inverted(liquidity, liquidityAssociation, condition)

	output := PearlOutput{
		Association:       association,
		Intervention:      intervention,
		InterventionScore: interventionScore,
		Uplift:            uplift,
		UpliftScore:       upliftScore,
		Contagion:         contagion,
		Condition:         condition,
		Inverted:          inverted,
	}
	output.score()

	if output.Strength <= 0 {
		return PearlOutput{}, false, nil
	}

	return output, true, nil
}

func (analyzer *pearlAnalyzer) column(index int) []float64 {
	values := make([]float64, 0, len(analyzer.rows))

	for _, row := range analyzer.rows {
		if len(row) <= index {
			continue
		}

		values = append(values, row[index])
	}

	return values
}

func (analyzer *pearlAnalyzer) uplift(flow []float64, target []float64) float64 {
	flowScale := analyzer.scale(flow)
	targetScale := analyzer.scale(target)

	if flowScale <= 0 || targetScale <= 0 {
		return 0
	}

	currentFlow := analyzer.current[pearlNodeFlow]
	counterfactualFlow := analyzer.percentile(flow, 1-1/float64(len(flow)))
	gap := math.Abs(counterfactualFlow - currentFlow)

	if gap <= 0 {
		return 0
	}

	return analyzer.slope(flow, target) * gap
}

func (analyzer *pearlAnalyzer) inverted(
	liquidity []float64,
	liquidityAssociation float64,
	condition float64,
) bool {
	currentLiquidity := analyzer.current[pearlNodeLiquidity]
	breakPoint := analyzer.center(liquidity) + analyzer.scale(liquidity)

	if currentLiquidity > 0 && breakPoint > 0 && currentLiquidity > breakPoint {
		return true
	}

	if condition > 0 && liquidityAssociation > analyzer.correlationMedian() {
		return true
	}

	return false
}

func (output *PearlOutput) score() {
	rungTotal := output.Association + output.InterventionScore + output.UpliftScore

	if output.Inverted {
		output.ShockScore = math.Max(output.Contagion, output.conditionShock(rungTotal))
		output.Strength = output.ShockScore

		return
	}

	counterfactualMass := 0.0

	if output.InterventionScore+output.UpliftScore > 0 {
		counterfactualMass = output.UpliftScore / (output.InterventionScore + output.UpliftScore)
	}

	output.AlphaScore = output.Association * output.InterventionScore * counterfactualMass
	output.BetaScore = output.Association * (1 - counterfactualMass)
	output.NoiseScore = output.noise(rungTotal)
	output.Strength = math.Max(
		output.AlphaScore,
		math.Max(output.BetaScore, math.Max(output.ShockScore, output.NoiseScore)),
	)
}

func (output PearlOutput) conditionShock(rungTotal float64) float64 {
	denominator := output.Condition + rungTotal + output.Contagion

	if denominator <= 0 {
		return 0
	}

	return output.Condition / denominator
}

func (output PearlOutput) noise(rungTotal float64) float64 {
	if rungTotal <= 0 {
		return 0
	}

	uncertainty := (1 - output.Association) /
		(1 + output.InterventionScore + output.UpliftScore)
	dominant := math.Max(
		output.Association,
		math.Max(output.InterventionScore, output.UpliftScore),
	)
	residual := rungTotal - dominant

	if residual <= 0 {
		return math.Max(uncertainty, 1/(1+dominant))
	}

	return math.Max(uncertainty, residual/(1+rungTotal))
}

func (analyzer *pearlAnalyzer) correlation(left []float64, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}

	leftScale := analyzer.scale(left)
	rightScale := analyzer.scale(right)

	if leftScale <= 0 || rightScale <= 0 {
		return 0
	}

	leftMean := analyzer.mean(left)
	rightMean := analyzer.mean(right)
	numerator := 0.0
	leftSum := 0.0
	rightSum := 0.0

	for index, leftValue := range left {
		rightValue := right[index]
		leftDelta := leftValue - leftMean
		rightDelta := rightValue - rightMean
		numerator += leftDelta * rightDelta
		leftSum += leftDelta * leftDelta
		rightSum += rightDelta * rightDelta
	}

	denominator := math.Sqrt(leftSum * rightSum)

	if denominator <= 0 {
		return 0
	}

	return analyzer.finite(numerator / denominator)
}

func (analyzer *pearlAnalyzer) slope(left []float64, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}

	leftMean := analyzer.mean(left)
	rightMean := analyzer.mean(right)
	numerator := 0.0
	denominator := 0.0

	for index, leftValue := range left {
		leftDelta := leftValue - leftMean
		numerator += leftDelta * (right[index] - rightMean)
		denominator += leftDelta * leftDelta
	}

	if denominator <= 0 {
		return 0
	}

	return analyzer.finite(numerator / denominator)
}

func (analyzer *pearlAnalyzer) condition(left []float64, right []float64) float64 {
	correlation := math.Abs(analyzer.correlation(left, right))

	if correlation <= 0 || correlation >= 1 {
		return 0
	}

	return correlation / (1 - correlation)
}

func (analyzer *pearlAnalyzer) correlationMedian() float64 {
	target := analyzer.column(pearlNodeTarget)
	values := []float64{
		math.Abs(analyzer.correlation(analyzer.column(pearlNodeMacro), target)),
		math.Abs(analyzer.correlation(analyzer.column(pearlNodeLiquidity), target)),
		math.Abs(analyzer.correlation(analyzer.column(pearlNodeFlow), target)),
	}

	return analyzer.center(values)
}

func (analyzer *pearlAnalyzer) scale(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	center := analyzer.center(values)
	deviations := make([]float64, 0, len(values))

	for _, value := range values {
		deviations = append(deviations, math.Abs(value-center))
	}

	scale := analyzer.center(deviations)

	if scale > 0 {
		return scale
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	scale = sorted[len(sorted)-1] - sorted[0]

	if scale <= 0 {
		return 0
	}

	return analyzer.finite(scale)
}

func (analyzer *pearlAnalyzer) center(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	return analyzer.percentile(values, 0.5)
}

func (analyzer *pearlAnalyzer) percentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	if percentile <= 0 {
		return sorted[0]
	}

	if percentile >= 1 {
		return sorted[len(sorted)-1]
	}

	position := percentile * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))

	if lower == upper {
		return sorted[lower]
	}

	weight := position - float64(lower)

	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func (analyzer *pearlAnalyzer) mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	total := 0.0

	for _, value := range values {
		total += value
	}

	return total / float64(len(values))
}

func (analyzer *pearlAnalyzer) finite(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	return value
}
