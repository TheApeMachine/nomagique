package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
PriceRing retains a bounded price history for return-based stages.
*/
type PriceRing struct {
	config *datura.Artifact
	bytes  []byte
}

/*
NewPriceRing returns a price ring stage configured on the artifact.
*/
func NewPriceRing(config *datura.Artifact) *PriceRing {
	return &PriceRing{
		config: config,
	}
}

func (priceRing *PriceRing) Write(payload []byte) (int, error) {
	priceRing.bytes = append(priceRing.bytes[:0], payload...)

	return len(payload), nil
}

func (priceRing *PriceRing) Read(payload []byte) (int, error) {
	state := datura.Acquire("price-ring-state", datura.APPJSON)

	if _, err := state.Write(priceRing.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	stageKey := datura.Peek[string](priceRing.config, "stage")

	if stageKey == "" {
		order := datura.Peek[[]string](priceRing.config, "order")
		stageIndex := int(datura.Peek[float64](priceRing.config, "inputs", "precursor", "stageIndex"))

		if stageIndex <= 0 {
			stageIndex = int(datura.Peek[float64](priceRing.config, "stageIndex"))
		}

		if stageIndex >= 0 && len(order) > stageIndex {
			stageKey = order[stageIndex]
		}
	}

	if stageKey == "" {
		return state.Read(payload)
	}

	rootKey := datura.Peek[string](state, "root")
	channelKeys := datura.Peek[[]string](state, "inputs")
	sourceKey := datura.Peek[string](priceRing.config, "inputs", stageKey, "input")
	sample := 0.0

	if rootKey != "" && sourceKey != "" && len(channelKeys) > 0 {
		for index, channelKey := range channelKeys {
			if channelKey != sourceKey {
				continue
			}

			sample = datura.Peek[float64](state, rootKey, index)

			if math.IsNaN(sample) || math.IsInf(sample, 0) {
				errnie.Error(errnie.Err(
					errnie.Validation,
					"price-ring: sample is NaN or Inf",
					nil,
				))
			}

			break
		}
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	returnLag := int(datura.Peek[float64](priceRing.config, "inputs", stageKey, "returnLag"))
	longHint := int(datura.Peek[float64](priceRing.config, "inputs", stageKey, "longWindow"))

	if returnLag <= 0 {
		returnLag = 1
	}

	prices := datura.Peek[[]float64](priceRing.config, "prices")
	prices = append(prices, sample)

	_, longWindow := RollingWindows(prices, 0, longHint)

	if longWindow > 0 && len(prices) > longWindow+returnLag {
		prices = prices[len(prices)-longWindow-returnLag:]
	}

	priceRing.config.Merge("prices", prices)

	state.Merge("sample", sample)

	return state.Read(payload)
}

func (priceRing *PriceRing) Close() error {
	return nil
}
