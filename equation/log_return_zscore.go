package equation

import (
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/statistic"
)

/*
LogReturnZScoreConfig configures precursor return normalization.
*/
type LogReturnZScoreConfig struct {
	ReturnLag    int
	LongWindow   int
	PositiveOnly bool
}

/*
LogReturnZScoreSample carries one positive price observation.
*/
type LogReturnZScoreSample struct {
	Series string
	Price  float64
	At     time.Time
}

/*
LogReturnZScoreOutput reports the normalized precursor score.
*/
type LogReturnZScoreOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
LogReturnZScore composes price validation, log return, z-score, and gating.
*/
type LogReturnZScore struct {
	priceRing     *statistic.PriceRing
	logReturn     *adaptive.LogReturn
	rollingZScore *statistic.RollingZScore
	positiveOnly  *adaptive.PositiveOnly
}

/*
NewLogReturnZScore returns a typed precursor calculator.
*/
func NewLogReturnZScore(
	config LogReturnZScoreConfig,
) (*LogReturnZScore, error) {
	if config.ReturnLag <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"log-return-zscore: return lag required",
			nil,
		))
	}

	return &LogReturnZScore{
		priceRing: statistic.NewPriceRing(),
		logReturn: adaptive.NewLogReturn(adaptive.LogReturnConfig{
			ReturnLag:  config.ReturnLag,
			LongWindow: config.LongWindow,
		}),
		rollingZScore: statistic.NewRollingZScore(),
		positiveOnly:  adaptive.NewPositiveOnly(config.PositiveOnly),
	}, nil
}

/*
Measure returns the latest gated log-return z-score.
*/
func (logReturnZScore *LogReturnZScore) Measure(
	sample LogReturnZScoreSample,
) (LogReturnZScoreOutput, error) {
	price, err := logReturnZScore.priceRing.Measure(sample.Price)
	if err != nil {
		return LogReturnZScoreOutput{}, err
	}

	logReturn, err := logReturnZScore.logReturn.Measure(adaptive.LogReturnSample{
		Series: sample.Series,
		Value:  price.Value,
		At:     sample.At,
	})
	if err != nil {
		return LogReturnZScoreOutput{}, err
	}

	if !logReturn.Ready {
		return LogReturnZScoreOutput{
			Ready: false,
			Count: logReturn.Count,
		}, nil
	}

	zscore, err := logReturnZScore.rollingZScore.Measure(statistic.TimedSample{
		Series: sample.Series,
		Value:  logReturn.Value,
		At:     sample.At,
	})
	if err != nil {
		return LogReturnZScoreOutput{}, err
	}

	positive, err := logReturnZScore.positiveOnly.Measure(zscore.Value)
	if err != nil {
		return LogReturnZScoreOutput{}, err
	}

	return LogReturnZScoreOutput{
		Value: positive.Value,
		Ready: zscore.Ready,
		Count: zscore.Count,
	}, nil
}
