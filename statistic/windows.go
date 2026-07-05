package statistic

import (
	"math"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
WindowsConfig provides optional adaptive window hints.
*/
type WindowsConfig struct {
	ShortHint     int
	LongHint      int
	ReturnLagHint int
}

/*
WindowsOutput reports resolved window counts.
*/
type WindowsOutput struct {
	ShortWindow int
	LongWindow  int
	ReturnLag   int
	TargetLong  int
}

/*
Windows resolves short, long, return-lag, and target-long counts from history.
*/
type Windows struct {
	config WindowsConfig
}

/*
NewWindows returns a typed window resolver.
*/
func NewWindows(configs ...WindowsConfig) *Windows {
	config := WindowsConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &Windows{
		config: config,
	}
}

/*
Measure resolves adaptive window counts from history.
*/
func (windows *Windows) Measure(history []float64) (WindowsOutput, error) {
	return ResolveWindowSet(history, windows.config)
}

/*
ResolveWindows returns short and long windows for imperative call sites.
*/
func ResolveWindows(
	history []float64,
	shortHint, longHint int,
) (shortWindow, longWindow int, err error) {
	output, err := ResolveWindowSet(history, WindowsConfig{
		ShortHint: shortHint,
		LongHint:  longHint,
	})

	if err != nil {
		return 0, 0, err
	}

	return output.ShortWindow, output.LongWindow, nil
}

/*
ResolveWindowSet resolves all window counts from history and optional hints.
*/
func ResolveWindowSet(history []float64, config WindowsConfig) (WindowsOutput, error) {
	sampleCount := len(history)
	shortWindow := config.ShortHint
	longWindow := config.LongHint

	if shortWindow > 0 && longWindow > 0 {
		return windowsOutput(shortWindow, longWindow, config.ReturnLagHint), nil
	}

	if sampleCount <= 0 {
		return WindowsOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"windows: history or explicit hints required",
			nil,
		))
	}

	for _, value := range history {
		if err := finiteStatistic("windows", value); err != nil {
			return WindowsOutput{}, err
		}
	}

	if shortWindow <= 0 {
		shortWindow = max(1, int(math.Ceil(math.Sqrt(float64(sampleCount)))))
	}

	if longWindow <= 0 {
		longWindow = adaptiveLongWindow(history, shortWindow)
	}

	targetLong := longWindow

	if config.LongHint <= 0 {
		targetLong = adaptiveLongWindow(history, shortWindow)
	}

	output := windowsOutput(shortWindow, longWindow, config.ReturnLagHint)
	output.TargetLong = targetLong

	return output, nil
}

func windowsOutput(shortWindow, longWindow, returnLagHint int) WindowsOutput {
	returnLag := returnLagHint

	if returnLag <= 0 {
		returnLag = max(1, int(math.Ceil(math.Sqrt(float64(longWindow)))))

		if longWindow > 1 {
			returnLag = min(returnLag, longWindow-1)
		}
	}

	return WindowsOutput{
		ShortWindow: shortWindow,
		LongWindow:  longWindow,
		ReturnLag:   returnLag,
		TargetLong:  longWindow,
	}
}

func adaptiveLongWindow(history []float64, shortWindow int) int {
	sampleCount := len(history)
	spread := 0.0

	if sampleCount >= 2 {
		mean := stat.Mean(history, nil)

		if mean > 0 && !math.IsNaN(mean) && !math.IsInf(mean, 0) {
			std := stat.StdDev(history, nil)

			if std > 0 && !math.IsNaN(std) && !math.IsInf(std, 0) {
				spread = std / math.Abs(mean)
			}
		}
	}

	longWindow := int(math.Ceil(float64(shortWindow) * (1.0 + spread)))

	if longWindow < sampleCount {
		longWindow = sampleCount
	}

	if longWindow <= shortWindow {
		longWindow = shortWindow + 1
	}

	if longWindow > sampleCount {
		longWindow = sampleCount
	}

	return longWindow
}
