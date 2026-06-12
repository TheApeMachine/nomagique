package adaptive

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel"
)

/*
Compressor scores how far below the running baseline the current sample sits.
*/
type Compressor struct {
	stageParser *core.StageParser
	state       kernel.CompressionState
}

/*
Compression returns a compression scorer ready from its first observation.
*/
func Compression() *Compressor {
	return &Compressor{
		stageParser: core.NewStageParser(),
	}
}

/*
Observe derives the compression score for the current sample.
*/
func (compressor *Compressor) Observe(
	inputs ...core.Number,
) core.Float64 {
	out, work, err := compressor.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	result, err := compressor.Apply(out, work)

	if err != nil {
		return 0
	}

	return result
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (compressor *Compressor) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	sample := float64(out)

	if len(work) > 0 {
		sample = float64(work[0])
	}

	return core.Float64(compressor.state.Observe(sample)), nil
}

/*
ObserveSamples runs the exact batch kernel over samples into out.
*/
func (compressor *Compressor) ObserveSamples(
	samples []float64, out []float64,
) {
	compressor.state.ObserveSamples(samples, out)
}

/*
Reset clears derived state.
*/
func (compressor *Compressor) Reset() error {
	compressor.state.Reset()
	return nil
}
