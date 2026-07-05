package algorithm

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
MoveBaselineConfig configures adaptive path-move thresholds.
*/
type MoveBaselineConfig struct {
	MinObs  int
	PathCap int
}

/*
MoveBaselineOutput reports anchor move readiness and stall margin.
*/
type MoveBaselineOutput struct {
	Value       float64
	Ready       float64
	Moved       float64
	StallMargin float64
	Mean        float64
	Variance    float64
	Count       int
	PathMoves   []float64
}

/*
MoveBaseline tracks adaptive path-move thresholds for anchor gating.
*/
type MoveBaseline struct {
	config    MoveBaselineConfig
	pathMoves []float64
	count     int
	mean      float64
	variance  float64
}

/*
NewMoveBaseline returns an anchor move gate.
*/
func NewMoveBaseline(config MoveBaselineConfig) *MoveBaseline {
	return &MoveBaseline{
		config: config,
	}
}

/*
Measure updates path-move history and reports whether the anchor is moving.
*/
func (baseline *MoveBaseline) Measure(recentMove float64) (MoveBaselineOutput, error) {
	if math.IsNaN(recentMove) || math.IsInf(recentMove, 0) {
		return MoveBaselineOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"move-baseline: sample must be finite",
			nil,
		))
	}

	if baseline.config.MinObs < 1 || baseline.config.PathCap < 1 {
		return MoveBaselineOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"move-baseline: minObs and pathCap required",
			nil,
		))
	}

	baseline.pathMoves = appendRingFloat(
		baseline.pathMoves,
		recentMove,
		baseline.config.PathCap,
	)

	effectiveAlpha := 2.0 / (float64(baseline.count + 1))

	if effectiveAlpha > 1 {
		effectiveAlpha = 1
	}

	moved := 0.0
	stallMargin := 0.0
	ready := 0.0
	minMove := pathMoveFloor(baseline.pathMoves, baseline.config.MinObs)

	if baseline.count < baseline.config.MinObs {
		baseline.mean, baseline.variance, baseline.count = updateMoveMoments(
			baseline.mean,
			baseline.variance,
			baseline.count,
			recentMove,
			effectiveAlpha,
		)

		return baseline.output(moved, stallMargin, ready), nil
	}

	historicalVar := baseline.variance

	if historicalVar < 0 {
		historicalVar = 0
	}

	floorVar := minMove * minMove
	threshold := baseline.mean + math.Sqrt(historicalVar+floorVar)

	if threshold <= 0 && recentMove <= 0 {
		ready = 1
		stallMargin = 1
	}

	if threshold > 0 || recentMove > 0 {
		ready = 1

		if recentMove > threshold {
			moved = 1
		}

		if moved == 0 && threshold > 0 {
			stallMargin = (threshold - recentMove) / threshold
		}
	}

	baseline.mean, baseline.variance, baseline.count = updateMoveMoments(
		baseline.mean,
		baseline.variance,
		baseline.count,
		recentMove,
		effectiveAlpha,
	)

	return baseline.output(moved, stallMargin, ready), nil
}

func (baseline *MoveBaseline) output(
	moved float64,
	stallMargin float64,
	ready float64,
) MoveBaselineOutput {
	return MoveBaselineOutput{
		Value:       ready,
		Ready:       ready,
		Moved:       moved,
		StallMargin: stallMargin,
		Mean:        baseline.mean,
		Variance:    baseline.variance,
		Count:       baseline.count,
		PathMoves:   append([]float64(nil), baseline.pathMoves...),
	}
}

func updateMoveMoments(
	mean, variance float64,
	count int,
	value, alpha float64,
) (float64, float64, int) {
	count++

	if count == 1 {
		return value, 0, count
	}

	delta := value - mean
	mean += alpha * delta
	variance = (1 - alpha) * (variance + alpha*delta*delta)

	return mean, variance, count
}

func pathMoveFloor(pathMoves []float64, minObs int) float64 {
	if len(pathMoves) < minObs {
		return 0
	}

	sortedMoves := append([]float64(nil), pathMoves...)
	sort.Float64s(sortedMoves)
	lowerRank := 1 / float64(len(sortedMoves))
	floor, err := moveSampleQuantile(lowerRank, sortedMoves)

	if err == nil && floor > 0 {
		return floor
	}

	for _, move := range pathMoves {
		if move <= 0 {
			continue
		}

		if floor == 0 || move < floor {
			floor = move
		}
	}

	return floor
}

func moveSampleQuantile(percentile float64, values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"move-baseline: quantile values required",
			nil,
		))
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	for _, value := range sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"move-baseline: quantile sample is non-finite",
				nil,
			))
		}
	}

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil), nil
}
