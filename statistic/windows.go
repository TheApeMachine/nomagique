package statistic

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/stat"
)

/*
RollingWindows resolves short and long lookback counts from history when hints are
non-positive. Positive hints are returned unchanged so callers can pin windows.
When history is empty and a hint is missing, RollingWindows returns an error.
*/
func RollingWindows(history []float64, shortHint, longHint int) (shortWindow, longWindow int, err error) {
	if shortHint > 0 {
		shortWindow = shortHint
	}

	if longHint > 0 {
		longWindow = longHint
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
		spread := coefficientOfVariation(history)
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
TargetLongWindow returns how many samples are required before a dynamic long-window
baseline is calibrated. Positive longWindow hints are returned unchanged.
*/
func TargetLongWindow(history []float64, shortHint, longHint int) (int, error) {
	if longHint > 0 {
		return longHint, nil
	}

	sampleCount := len(history)

	if sampleCount <= 0 {
		_, longWindow, err := RollingWindows(nil, shortHint, longHint)

		return longWindow, err
	}

	shortWindow := shortHint

	if shortWindow <= 0 {
		shortWindow = int(math.Ceil(math.Sqrt(float64(sampleCount))))

		if shortWindow < 1 {
			shortWindow = 1
		}
	}

	spread := coefficientOfVariation(history)
	longWindow := int(math.Ceil(float64(shortWindow) * (1.0 + spread)))

	if longWindow < sampleCount {
		longWindow = sampleCount
	}

	if longWindow <= shortWindow {
		longWindow = shortWindow + 1
	}

	return longWindow, nil
}

func coefficientOfVariation(values []float64) float64 {
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
