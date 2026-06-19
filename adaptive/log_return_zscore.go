package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
LogReturnZScore scores log-return distance from the rolling return distribution.
*/
type LogReturnZScore struct {
	config *datura.Artifact
	staged *datura.Artifact
}

/*
NewLogReturnZScore returns a log-return z-score stage configured on the artifact.
*/
func NewLogReturnZScore(config *datura.Artifact) *LogReturnZScore {
	return &LogReturnZScore{
		config: config,
		staged: datura.Acquire("log-return-zscore", datura.APPJSON),
	}
}

func (logReturnZScore *LogReturnZScore) Write(payload []byte) (int, error) {
	return logReturnZScore.staged.Write(payload)
}

func (logReturnZScore *LogReturnZScore) Read(payload []byte) (int, error) {
	stageKey := logReturnZScore.stageKey()

	if stageKey == "" {
		return logReturnZScore.staged.Read(payload)
	}

	rootKey := datura.Peek[string](logReturnZScore.staged, "root")
	channelKeys := datura.Peek[[]string](logReturnZScore.staged, "inputs")
	sourceKey := datura.Peek[string](logReturnZScore.config, "inputs", stageKey, "input")

	sample := logReturnZScore.sample(logReturnZScore.staged, rootKey, channelKeys, sourceKey)

	returnLag := int(datura.Peek[float64](logReturnZScore.config, "inputs", stageKey, "returnLag"))
	longWindow := int(datura.Peek[float64](logReturnZScore.config, "inputs", stageKey, "longWindow"))
	outputKey := datura.Peek[string](logReturnZScore.config, "inputs", stageKey, "outputKey")

	if returnLag <= 0 || longWindow <= 0 || outputKey == "" {
		return logReturnZScore.staged.Read(payload)
	}

	if sample <= 0 || math.IsNaN(sample) || math.IsInf(sample, 0) {
		return logReturnZScore.staged.Read(payload)
	}

	prices := datura.Peek[[]float64](logReturnZScore.staged, "state", "prices")
	prices = append(prices, sample)

	if len(prices) > longWindow+returnLag {
		prices = prices[len(prices)-longWindow-returnLag:]
	}

	logReturnZScore.staged.Poke(prices, "state", "prices")

	score := 0.0

	if len(prices) > returnLag {
		anchorPrice := prices[len(prices)-returnLag-1]

		if anchorPrice > 0 {
			logReturn := math.Log(sample / anchorPrice)
			returns := datura.Peek[[]float64](logReturnZScore.staged, "state", "returns")
			returns = append(returns, logReturn)

			if len(returns) > longWindow {
				returns = returns[len(returns)-longWindow:]
			}

			logReturnZScore.staged.Poke(returns, "state", "returns")

			if len(returns) >= 2 {
				meanReturn := stat.Mean(returns, nil)
				stdReturn := stat.StdDev(returns, nil)

				if stdReturn > 0 {
					score = (logReturn - meanReturn) / stdReturn
				}
			}
		}
	}

	if datura.Peek[float64](logReturnZScore.config, "inputs", stageKey, "positiveOnly") > 0 {
		score = math.Max(0, score)
	}

	logReturnZScore.staged.Poke(score, "output", outputKey)

	return logReturnZScore.staged.Read(payload)
}

func (logReturnZScore *LogReturnZScore) stageKey() string {
	order := datura.Peek[[]string](logReturnZScore.config, "order")

	if len(order) < 2 {
		return ""
	}

	return order[1]
}

func (logReturnZScore *LogReturnZScore) sample(
	artifact *datura.Artifact,
	rootKey string,
	channelKeys []string,
	sourceKey string,
) float64 {
	if rootKey == "" || sourceKey == "" || len(channelKeys) == 0 {
		return 0
	}

	for index, channelKey := range channelKeys {
		if channelKey != sourceKey {
			continue
		}

		return datura.Peek[float64](artifact, rootKey, index)
	}

	return 0
}

func (logReturnZScore *LogReturnZScore) Close() error {
	return nil
}
