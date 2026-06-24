package equation

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/geometry"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/vector"
)

/*
Ignition composes volume lift, precursor, spread compression, and logit scoring stages.
*/
type Ignition struct {
	artifact *datura.Artifact
	pipeline io.ReadWriteCloser
}

/*
NewIgnition composes ignition stages from one shared config artifact.
*/
func NewIgnition(artifact *datura.Artifact) *Ignition {
	return &Ignition{
		artifact: artifact,
		pipeline: transport.NewPipeline(
			statistic.NewMeanMedianRatio(artifact),
			NewLogReturnZScore(artifact),
			vector.NewSpreadSample(artifact),
			adaptive.NewCompression(artifact),
			geometry.NewGeometricMean(artifact),
			learning.NewLogitScores(artifact),
		),
	}
}

func (ignition *Ignition) Read(p []byte) (int, error) {
	return ignition.pipeline.Read(p)
}

func (ignition *Ignition) Write(p []byte) (int, error) {
	return ignition.pipeline.Write(p)
}

func (ignition *Ignition) Close() error {
	return ignition.pipeline.Close()
}
