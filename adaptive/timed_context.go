package adaptive

import (
	"math"
	"time"

	"github.com/theapemachine/nomagique/statistic"
)

/*
TimedContext derives windows and smoothing constants from observation rings.
*/
type TimedContext struct {
	TradeIntervals     *statistic.ObservationRing
	LevelLifetimes     *statistic.ObservationRing
	BookPulseIntervals *statistic.ObservationRing
	ChurnDurations     *statistic.ObservationRing
}

func NewTimedContext() *TimedContext {
	return &TimedContext{
		TradeIntervals:     statistic.NewObservationRing(),
		LevelLifetimes:     statistic.NewObservationRing(),
		BookPulseIntervals: statistic.NewObservationRing(),
		ChurnDurations:     statistic.NewObservationRing(),
	}
}

func (timedContext *TimedContext) MatchWindow(tradeSpan time.Duration) time.Duration {
	if tradeSpan > 0 {
		return tradeSpan
	}

	if timedContext.TradeIntervals.Len() >= 3 {
		median := timedContext.TradeIntervals.Median()
		p75 := timedContext.TradeIntervals.Quantile(0.75)

		return time.Duration((median + p75) * float64(time.Second))
	}

	if timedContext.BookPulseIntervals.Len() >= 3 {
		median := timedContext.BookPulseIntervals.Median()

		return time.Duration(median * float64(time.Second))
	}

	return 0
}

func (timedContext *TimedContext) MaxAge() time.Duration {
	if timedContext.LevelLifetimes.Len() >= 3 {
		p75 := timedContext.LevelLifetimes.Quantile(0.75)

		return time.Duration(p75 * float64(time.Second))
	}

	if timedContext.BookPulseIntervals.Len() >= 3 {
		median := timedContext.BookPulseIntervals.Median()

		return time.Duration(median * float64(time.Second))
	}

	return 0
}

func (timedContext *TimedContext) Cooldown(matchWindow time.Duration) time.Duration {
	maxAge := timedContext.MaxAge()

	if maxAge > 0 && matchWindow > 0 {
		return maxAge + matchWindow
	}

	if maxAge > 0 {
		return maxAge
	}

	if matchWindow > 0 {
		return matchWindow
	}

	if timedContext.ChurnDurations.Len() >= 3 {
		p75 := timedContext.ChurnDurations.Quantile(0.75)

		return time.Duration(p75 * float64(time.Second))
	}

	return 0
}

func (timedContext *TimedContext) FlashWindow() time.Duration {
	if timedContext.ChurnDurations.Len() >= 3 {
		p75 := timedContext.ChurnDurations.Quantile(0.75)

		return time.Duration(p75 * float64(time.Second))
	}

	if timedContext.BookPulseIntervals.Len() >= 3 {
		median := timedContext.BookPulseIntervals.Median()

		return time.Duration(median * float64(time.Second))
	}

	return 0
}

func (timedContext *TimedContext) TradeRetentionCount() int {
	if timedContext.TradeIntervals.Len() >= 3 {
		span := timedContext.TradeIntervals.Span()
		median := timedContext.TradeIntervals.Median()

		if median > 0 && span > 0 {
			return int(math.Ceil(span/median)) + 1
		}
	}

	return 1
}

func (timedContext *TimedContext) MeanTradeIntervalSeconds(tradeSpan time.Duration, tradeCount int) float64 {
	if timedContext.TradeIntervals.Len() >= 1 {
		return timedContext.TradeIntervals.Median()
	}

	if tradeCount >= 2 && tradeSpan > 0 {
		count := float64(tradeCount - 1)

		if count > 0 {
			return tradeSpan.Seconds() / count
		}
	}

	if timedContext.BookPulseIntervals.Len() >= 1 {
		return timedContext.BookPulseIntervals.Median()
	}

	return 0
}

func (timedContext *TimedContext) FlowSmoothingWindow(
	matchWindow time.Duration,
	tradeSpan time.Duration,
) time.Duration {
	maxAge := timedContext.MaxAge()

	if matchWindow > 0 && maxAge > 0 {
		return matchWindow + maxAge
	}

	if matchWindow > 0 {
		return matchWindow
	}

	if maxAge > 0 {
		return maxAge
	}

	if tradeSpan > 0 {
		return tradeSpan
	}

	if timedContext.BookPulseIntervals.Len() >= 1 {
		median := timedContext.BookPulseIntervals.Median()

		return time.Duration(median * float64(time.Second))
	}

	return 0
}

func (timedContext *TimedContext) FlowSmoothingAlpha(
	matchWindow time.Duration,
	tradeSpan time.Duration,
	tradeCount int,
) float64 {
	meanInterval := timedContext.MeanTradeIntervalSeconds(tradeSpan, tradeCount)
	window := timedContext.FlowSmoothingWindow(matchWindow, tradeSpan)

	if meanInterval <= 0 || window <= 0 {
		return 0
	}

	windowSeconds := float64(window) / float64(time.Second)

	return windowSeconds / (meanInterval + windowSeconds)
}
