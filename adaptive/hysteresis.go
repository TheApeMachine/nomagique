package adaptive

import "github.com/theapemachine/errnie"

/*
HysteresisConfig configures the debounce gate.
*/
type HysteresisConfig struct {
	Window    int
	Threshold float64
}

/*
Hysteresis debounces a binary signal so brief trips do not flip state.
*/
type Hysteresis struct {
	config      HysteresisConfig
	value       float64
	pendingHigh int
	pendingLow  int
	count       int
}

/*
HysteresisOutput reports the debounced binary state.
*/
type HysteresisOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewHysteresis returns a typed hysteresis gate.
*/
func NewHysteresis(config HysteresisConfig) *Hysteresis {
	return &Hysteresis{
		config: config,
	}
}

/*
Measure adds one sample and returns the debounced binary state.
*/
func (hysteresis *Hysteresis) Measure(sample float64) (HysteresisOutput, error) {
	if hysteresis.config.Window <= 0 {
		return HysteresisOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"hysteresis: window required",
			nil,
		))
	}

	if err := finiteAdaptive("hysteresis", sample); err != nil {
		return HysteresisOutput{}, err
	}

	if sample > hysteresis.config.Threshold {
		hysteresis.pendingHigh++
		hysteresis.pendingLow = 0

		if hysteresis.pendingHigh >= hysteresis.config.Window {
			hysteresis.value = 1
		}
	}

	if sample <= hysteresis.config.Threshold {
		hysteresis.pendingLow++
		hysteresis.pendingHigh = 0

		if hysteresis.pendingLow >= hysteresis.config.Window {
			hysteresis.value = 0
		}
	}

	hysteresis.count++

	return HysteresisOutput{
		Value: hysteresis.value,
		Ready: true,
		Count: hysteresis.count,
	}, nil
}
