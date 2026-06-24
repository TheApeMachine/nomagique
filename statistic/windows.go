package statistic

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/stat"
)

/*
RollingWindow resolves short and long lookback counts from history when hints are
non-positive. Positive hints are returned unchanged so callers can pin windows.
When history is empty and a hint is missing, Resolve returns an error.
*/
type RollingWindow struct {
	shortHint int
	longHint  int
}

/*
NewRollingWindow binds optional short and long window hints from config.
*/
func NewRollingWindow(shortHint, longHint int) *RollingWindow {
	return &RollingWindow{
		shortHint: shortHint,
		longHint:  longHint,
	}
}

/*
Resolve derives short and long window sizes from bound hints and sample history.
*/
func (rolling *RollingWindow) Resolve(history []float64) (shortWindow, longWindow int, err error) {
	if rolling.shortHint > 0 {
		shortWindow = rolling.shortHint
	}

	if rolling.longHint > 0 {
		longWindow = rolling.longHint
	}

	if shortWindow > 0 && longWindow > 0 {
		return shortWindow, longWindow, nil
	}

	sampleCount := len(history)

	if sampleCount <= 0 {
		return 0, 0, fmt.Errorf("statistic: rolling windows require history or explicit hints")
	}

	if shortWindow <= 0 {
		shortWindow = int(math.Ceil(math.Sqrt(float64(sampleCount))))

		if shortWindow < 1 {
			shortWindow = 1
		}
	}

	if longWindow <= 0 {
		spread := rolling.coefficientOfVariation(history)
		longWindow = int(math.Ceil(float64(shortWindow) * (1.0 + spread)))

		if longWindow < sampleCount {
			longWindow = sampleCount
		}

		if longWindow <= shortWindow {
			longWindow = shortWindow + 1
		}

		if longWindow > sampleCount {
			longWindow = sampleCount
		}
	}

	return shortWindow, longWindow, nil
}

/*
TargetLong returns how many samples are required before a dynamic long-window
baseline is calibrated. Positive long hints are returned unchanged.
*/
func (rolling *RollingWindow) TargetLong(history []float64) (int, error) {
	if rolling.longHint > 0 {
		return rolling.longHint, nil
	}

	sampleCount := len(history)

	if sampleCount <= 0 {
		_, longWindow, err := NewRollingWindow(rolling.shortHint, rolling.longHint).Resolve(nil)

		return longWindow, err
	}

	shortWindow := rolling.shortHint

	if shortWindow <= 0 {
		shortWindow = int(math.Ceil(math.Sqrt(float64(sampleCount))))

		if shortWindow < 1 {
			shortWindow = 1
		}
	}

	spread := rolling.coefficientOfVariation(history)
	longWindow := int(math.Ceil(float64(shortWindow) * (1.0 + spread)))

	if longWindow < sampleCount {
		longWindow = sampleCount
	}

	if longWindow <= shortWindow {
		longWindow = shortWindow + 1
	}

	if longWindow > sampleCount && sampleCount > shortWindow {
		longWindow = sampleCount
	}

	return longWindow, nil
}

/*
ReturnLag resolves a log-return lag from history when the hint is non-positive.
Positive hints are returned unchanged.
*/
func (rolling *RollingWindow) ReturnLag(history []float64, lagHint int) (int, error) {
	if lagHint > 0 {
		return lagHint, nil
	}

	if len(history) == 0 {
		return 1, nil
	}

	_, longWindow, err := NewRollingWindow(0, rolling.longHint).Resolve(history)

	if err != nil {
		return 0, err
	}

	lag := int(math.Ceil(math.Sqrt(float64(longWindow))))

	if lag < 1 {
		lag = 1
	}

	if longWindow > 1 && lag >= longWindow {
		lag = longWindow - 1
	}

	return lag, nil
}

func (rolling *RollingWindow) coefficientOfVariation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	mean := stat.Mean(values, nil)

	if mean <= 0 || math.IsNaN(mean) || math.IsInf(mean, 0) {
		return 0
	}

	std := stat.StdDev(values, nil)

	if std <= 0 || math.IsNaN(std) || math.IsInf(std, 0) {
		return 0
	}

	return std / math.Abs(mean)
}
