package statistic

import (
	"math"

	"gonum.org/v1/gonum/stat"
)

/*
RollingWindows resolves short and long lookback counts from history when hints are
non-positive. Positive hints are returned unchanged so tests can pin windows.
*/
func RollingWindows(history []float64, shortHint, longHint int) (shortWindow, longWindow int) {
	if shortHint > 0 {
		shortWindow = shortHint
	}

	if longHint > 0 {
		longWindow = longHint
	}

	if shortWindow > 0 && longWindow > 0 {
		return shortWindow, longWindow
	}

	sampleCount := len(history)

	if sampleCount <= 0 {
		if shortWindow <= 0 {
			shortWindow = 2
		}

		if longWindow <= 0 {
			longWindow = shortWindow + 1
		}

		return shortWindow, longWindow
	}

	if shortWindow <= 0 {
		shortWindow = int(math.Ceil(math.Sqrt(float64(sampleCount))))

		if shortWindow < 2 {
			shortWindow = 2
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

	return shortWindow, longWindow
}

/*
TargetLongWindow returns how many samples are required before a dynamic long-window
baseline is calibrated. Positive longWindow hints are returned unchanged.
*/
func TargetLongWindow(history []float64, shortHint, longHint int) int {
	if longHint > 0 {
		return longHint
	}

	sampleCount := len(history)

	if sampleCount <= 0 {
		_, longWindow := RollingWindows(nil, shortHint, longHint)

		return longWindow
	}

	shortWindow := shortHint

	if shortWindow <= 0 {
		shortWindow = int(math.Ceil(math.Sqrt(float64(sampleCount))))

		if shortWindow < 2 {
			shortWindow = 2
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

	return longWindow
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
