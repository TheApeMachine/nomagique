package equation

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/statistic"
)

/*
LogReturnZScore composes price retention, log return, rolling z-score, and
optional positive-only gating into the precursor mini-pipeline.
*/
type LogReturnZScore struct {
	inner io.ReadWriteCloser
}

/*
NewLogReturnZScore composes the precursor mini-pipeline from config attributes.
*/
func NewLogReturnZScore(config *datura.Artifact) io.ReadWriteCloser {
	stageConfig := config

	if cloned, err := config.Clone(); err == nil {
		stageConfig = cloned
	}

	if datura.Peek[string](stageConfig, "stage") == "" {
		stageConfig.Poke("precursor", "stage")
	}

	return &LogReturnZScore{
		inner: transport.NewPipeline(
			statistic.NewPriceRing(stageConfig),
			adaptive.NewLogReturn(stageConfig),
			statistic.NewRollingZScore(stageConfig),
			adaptive.NewPositiveOnly(stageConfig),
		),
	}
}

func (logReturnZScore *LogReturnZScore) Write(payload []byte) (int, error) {
	return logReturnZScore.inner.Write(payload)
}

func (logReturnZScore *LogReturnZScore) Read(payload []byte) (int, error) {
	return logReturnZScore.inner.Read(payload)
}

func (logReturnZScore *LogReturnZScore) Close() error {
	return logReturnZScore.inner.Close()
}
