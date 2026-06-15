package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/causal"
)

/*
Pearl implements Judea Pearl's ladder of causation over a tabular structural model.
*/
type Pearl struct {
	artifact  *datura.Artifact
	target    int
	config    causal.LadderConfig
	nodes     *NodeRing
	contagion io.ReadWriter
	current   []float64
	tracker   *causal.RegimeTracker
	weights   []float64
	outcome   causal.Outcome
}

/*
NewPearl creates a Pearl stage over a NodeRing history and a contagion scalar stage.
*/
func NewPearl(
	target int,
	config causal.LadderConfig,
	nodes *NodeRing,
	contagion io.ReadWriter,
	weights []float64,
) *Pearl {
	if config.MinHistory <= 0 {
		config.MinHistory = 12
	}

	streams := [][]float64(nil)

	if nodes != nil {
		streams = nodes.Streams()
	}

	config = applyDerivedLadderConfig(config, streams)

	return &Pearl{
		artifact:  datura.Acquire("pearl", datura.Artifact_Type_json),
		target:    target,
		config:    config,
		nodes:     nodes,
		contagion: contagion,
		tracker:   causal.NewRegimeTracker(),
		weights:   weights,
	}
}

func (pearl *Pearl) Write(p []byte) (int, error) {
	return pearl.artifact.Write(p)
}

func (pearl *Pearl) Read(p []byte) (int, error) {
	rehydrateArtifact(&pearl.artifact, "pearl", datura.Artifact_Type_json)

	payload, err := pearl.artifact.Payload()

	if err == nil {
		batch := payloadSamples(payload)

		if len(batch) > 0 {
			pearl.current = batch
		}
	}

	table, currentRow, ok := pearl.tableFromStreams()

	if ok {
		contagion := pearl.readContagion()
		pearl.outcome = causal.Evaluate(
			table, currentRow, contagion, pearl.config, pearl.tracker,
		)

		out := encodePayload(pearl.outcome.Raw)
		_ = pearl.artifact.SetPayload(out)
		pearl.publishReadings()
	}

	return pearl.artifact.Read(p)
}

func (pearl *Pearl) Close() error {
	return nil
}

/*
Association returns the Pearson association between treatment and target.
*/
func (pearl *Pearl) Association() float64 {
	return pearl.outcome.Association
}

/*
Intervention returns the kernel backdoor intervention estimate.
*/
func (pearl *Pearl) Intervention() float64 {
	return pearl.outcome.Intervention
}

/*
Uplift returns the nonlinear counterfactual uplift when available.
*/
func (pearl *Pearl) Uplift() float64 {
	return pearl.outcome.Uplift
}

/*
RegimeInverted reports whether the inverted role set is active.
*/
func (pearl *Pearl) RegimeInverted() bool {
	return pearl.outcome.Inverted
}

/*
Outcome returns the full ladder decomposition from the last Read call.
*/
func (pearl *Pearl) Outcome() causal.Outcome {
	return pearl.outcome
}

/*
SetNodes binds the live node history used for tabular ladder evaluation.
*/
func (pearl *Pearl) SetNodes(nodes *NodeRing) {
	pearl.nodes = nodes

	if nodes == nil {
		return
	}

	alignedLength := nodes.AlignedLength()

	if alignedLength > 0 {
		pearl.config.MinHistory = alignedLength
	}

	pearl.config = applyDerivedLadderConfig(pearl.config, nodes.Streams())
}

/*
Reset clears derived state.
*/
func (pearl *Pearl) Reset() error {
	pearl.weights = nil
	pearl.current = nil
	pearl.outcome = causal.Outcome{}
	pearl.tracker = causal.NewRegimeTracker()

	return nil
}

/*
AssociationReading returns the association rung as a composable score source.
*/
func (pearl *Pearl) AssociationReading() *PearlReading {
	return newPearlReading(pearl, func(outcome causal.Outcome) float64 {
		return outcome.Association
	})
}

/*
InterventionReading returns the intervention rung as a composable score source.
*/
func (pearl *Pearl) InterventionReading() *PearlReading {
	return newPearlReading(pearl, func(outcome causal.Outcome) float64 {
		return outcome.Intervention
	})
}

/*
UpliftReading returns the counterfactual uplift as a composable score source.
*/
func (pearl *Pearl) UpliftReading() *PearlReading {
	return newPearlReading(pearl, func(outcome causal.Outcome) float64 {
		return outcome.Uplift
	})
}

/*
ContagionReading returns the contagion magnitude as a composable score source.
*/
func (pearl *Pearl) ContagionReading() *PearlReading {
	return newPearlReading(pearl, func(outcome causal.Outcome) float64 {
		return outcome.Contagion
	})
}

/*
PearlReading exposes one causal.Outcome field as a pipeline score source.
*/
type PearlReading struct {
	artifact *datura.Artifact
	pearl    *Pearl
	project  func(causal.Outcome) float64
}

func newPearlReading(
	pearl *Pearl,
	project func(causal.Outcome) float64,
) *PearlReading {
	return &PearlReading{
		artifact: datura.Acquire("pearl-reading", datura.Artifact_Type_json),
		pearl:    pearl,
		project:  project,
	}
}

func (reading *PearlReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *PearlReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.pearl != nil && reading.project != nil {
		value = reading.project(reading.pearl.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *PearlReading) Close() error {
	return nil
}

func (pearl *Pearl) publishReadings() {
	pokeFloat(pearl.artifact, "ladder.uplift", pearl.outcome.Uplift)
	pokeFloat(pearl.artifact, "ladder.contagion", pearl.outcome.Contagion)
	pokeFloat(pearl.artifact, "ladder.association", pearl.outcome.Association)
	pokeFloat(pearl.artifact, "ladder.intervention", pearl.outcome.Intervention)
}

func (pearl *Pearl) readContagion() float64 {
	if pearl.contagion == nil {
		return 0
	}

	outBuf := make([]byte, 4096)
	_, readErr := pearl.contagion.Read(outBuf)

	if readErr != nil && readErr != io.EOF {
		return 0
	}

	outbound := datura.Acquire("pearl-contagion", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf)
	payload, payloadErr := outbound.Payload()

	if payloadErr != nil {
		return 0
	}

	value, ok := payloadScalar(payload)

	if !ok {
		return 0
	}

	return value
}

func (pearl *Pearl) tableFromStreams() (causal.NodeTable, []float64, bool) {
	streams := [][]float64(nil)

	if pearl.nodes != nil {
		streams = pearl.nodes.Streams()
	}

	rows, ok := zipNodeRows(streams)

	if !ok {
		return causal.NodeTable{}, nil, false
	}

	currentRow := pearl.currentRow(rows)

	table, err := causal.NewNodeTable(rows, pearl.target, pearl.config.MinHistory)

	if err != nil {
		return causal.NodeTable{}, nil, false
	}

	return table, currentRow, true
}

func (pearl *Pearl) currentRow(rows [][]float64) []float64 {
	if len(pearl.current) == len(rows[0]) {
		return pearl.current
	}

	if len(rows) == 0 {
		return nil
	}

	return rows[len(rows)-1]
}
