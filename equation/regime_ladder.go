package equation

import (
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/causal"
)

/*
RegimeLadderConfig composes regime selection, hysteresis, and ladder evaluation.
*/
type RegimeLadderConfig struct {
	Regime     causal.RegimeConfig
	Hysteresis adaptive.HysteresisConfig
	Ladder     causal.LadderConfig
}

/*
RegimeLadderSample carries retained causal rows and contagion context.
*/
type RegimeLadderSample struct {
	Rows      [][]float64
	Contagion float64
}

/*
RegimeLadderOutput reports typed ladder evidence.
*/
type RegimeLadderOutput = causal.LadderOutput

/*
RegimeLadder composes regime selection, hysteresis smoothing, and ladder evaluation.
*/
type RegimeLadder struct {
	regime     *causal.Regime
	hysteresis *adaptive.Hysteresis
	ladder     *causal.Ladder
}

/*
NewRegimeLadder returns a typed regime-ladder evaluator.
*/
func NewRegimeLadder(config RegimeLadderConfig) (*RegimeLadder, error) {
	if config.Hysteresis.Window <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"regime-ladder: hysteresis window required",
			nil,
		))
	}

	return &RegimeLadder{
		regime:     causal.NewRegime(config.Regime),
		hysteresis: adaptive.NewHysteresis(config.Hysteresis),
		ladder:     causal.NewLadder(config.Ladder),
	}, nil
}

/*
Measure evaluates the current regime and ladder evidence.
*/
func (regimeLadder *RegimeLadder) Measure(
	sample RegimeLadderSample,
) (RegimeLadderOutput, error) {
	regime, err := regimeLadder.regime.Measure(causal.RegimeInput{
		Rows:      sample.Rows,
		Contagion: sample.Contagion,
	})
	if err != nil {
		return RegimeLadderOutput{}, err
	}

	hysteresis, err := regimeLadder.hysteresis.Measure(regime.RawInverted)
	if err != nil {
		return RegimeLadderOutput{}, err
	}

	return regimeLadder.ladder.Measure(causal.LadderInput{
		Rows:      sample.Rows,
		Inverted:  hysteresis.Value != 0,
		Contagion: sample.Contagion,
		Condition: regime.Condition,
	})
}
