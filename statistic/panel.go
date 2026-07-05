package statistic

import (
	"maps"

	"github.com/theapemachine/errnie"
)

/*
Panel registers keyed samples for cross-section pipelines.
*/
type Panel struct {
	peers map[string]float64
}

/*
NewPanel returns a typed keyed sample registry.
*/
func NewPanel() *Panel {
	return &Panel{
		peers: map[string]float64{},
	}
}

/*
Measure stores one member sample and returns a snapshot of the panel.
*/
func (panel *Panel) Measure(sample PanelSample) (PanelOutput, error) {
	if sample.Member == "" {
		return PanelOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"panel: member required",
			nil,
		))
	}

	if err := finiteStatistic("panel", sample.Value); err != nil {
		return PanelOutput{}, err
	}

	panel.peers[sample.Member] = sample.Value

	return PanelOutput{
		Peers: maps.Clone(panel.peers),
		Value: sample.Value,
		Count: len(panel.peers),
	}, nil
}
