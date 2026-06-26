package algorithm

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
MoveBaseline tracks adaptive path-move thresholds for anchor gating.
*/
type MoveBaseline struct {
	artifact *datura.Artifact
}

/*
NewMoveBaseline returns an anchor move gate wired from config on the artifact.
*/
func NewMoveBaseline(artifact *datura.Artifact) *MoveBaseline {
	return &MoveBaseline{
		artifact: artifact,
	}
}

func (baseline *MoveBaseline) Read(payload []byte) (int, error) {
	state := datura.Acquire("move-baseline-state", datura.APPJSON)

	if _, err := state.Unpack(baseline.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"move-baseline: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](baseline.artifact, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"move-baseline: root required",
			nil,
		))
	}

	sampleKey := datura.Peek[string](baseline.artifact, "sampleKey")

	if sampleKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"move-baseline: sampleKey required",
			nil,
		))
	}

	recentMove := datura.Peek[float64](state, rootKey, sampleKey)

	minObs := int(datura.Peek[float64](baseline.artifact, "minObs"))
	pathCap := int(datura.Peek[float64](baseline.artifact, "pathCap"))

	if minObs < 1 || pathCap < 1 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"move-baseline: minObs and pathCap required",
			nil,
		))
	}

	pathMoves := datura.Peek[[]float64](baseline.artifact, "output", "pathMoves")
	count := int(datura.Peek[float64](baseline.artifact, "output", "count"))
	mean := datura.Peek[float64](baseline.artifact, "output", "mean")
	variance := datura.Peek[float64](baseline.artifact, "output", "variance")

	pathMoves = appendRingFloat(pathMoves, recentMove, pathCap)
	effectiveAlpha := 2.0 / (float64(count + 1))

	if effectiveAlpha > 1 {
		effectiveAlpha = 1
	}

	minMove := pathMoveFloor(pathMoves, minObs)
	moved := 0.0
	stallMargin := 0.0
	ready := 0.0

	if count < minObs {
		mean, variance, count = updateMoveMoments(mean, variance, count, recentMove, effectiveAlpha)
	} else {
		historicalVar := variance

		if historicalVar < 0 {
			historicalVar = 0
		}

		floorVar := minMove * minMove
		threshold := mean + math.Sqrt(historicalVar+floorVar)

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

		mean, variance, count = updateMoveMoments(mean, variance, count, recentMove, effectiveAlpha)
	}

	baseline.artifact.Poke(pathMoves, "output", "pathMoves")
	baseline.artifact.Poke(float64(count), "output", "count")
	baseline.artifact.Poke(mean, "output", "mean")
	baseline.artifact.Poke(variance, "output", "variance")
	baseline.artifact.Poke(moved, "output", "moved")
	baseline.artifact.Poke(stallMargin, "output", "stallMargin")
	baseline.artifact.Poke(ready, "output", "ready")
	baseline.artifact.Poke(ready, "output", "value")
	state.MergeOutput("moved", moved)
	state.MergeOutput("stallMargin", stallMargin)
	state.MergeOutput("ready", ready)
	state.MergeOutput("value", ready)
	state.Poke("output", "root")
	state.Poke([]string{"moved", "stallMargin", "ready", "value"}, "inputs")

	return state.PackInto(payload)
}

func (baseline *MoveBaseline) Write(payload []byte) (int, error) {
	baseline.artifact.WithPayload(payload)
	return len(payload), nil
}

func (baseline *MoveBaseline) Close() error {
	return nil
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
