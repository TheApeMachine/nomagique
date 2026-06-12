package core

/*
Pipeline runs an ordered list of numbers over a raw boundary sample.
*/
type Pipeline struct {
	registry *BoundaryRegistry
	stages   []Number
	scratch  []Number
	work     []Float64
	workLen  int
}

/*
NewPipeline builds a pipeline from the given stages using the default registry.
*/
func NewPipeline(stages []Number) *Pipeline {
	return NewPipelineWithRegistry(DefaultRegistry, stages)
}

/*
NewPipelineWithRegistry builds a pipeline bound to registry.
*/
func NewPipelineWithRegistry(
	registry *BoundaryRegistry, stages []Number,
) *Pipeline {
	return &Pipeline{
		registry: registry,
		stages:   registry.ExpandNumbers(stages),
		scratch:  make([]Number, 0, 8),
		work:     make([]Float64, 0, 8),
	}
}

/*
Bind reconfigures a pooled pipeline for new stages.
*/
func (pipeline *Pipeline) Bind(stages []Number) {
	pipeline.stages = pipeline.registry.ExpandNumbers(stages)
	pipeline.workLen = 0
	pipeline.work = pipeline.work[:0]
	pipeline.scratch = pipeline.scratch[:0]
}

/*
Observe runs each stage against the raw sample carried through work.
*/
func (pipeline *Pipeline) Observe(raw Float64) (Float64, error) {
	stageOut := Float64(0)
	pipeline.workLen = 1

	if cap(pipeline.work) < 1 {
		pipeline.work = make([]Float64, 0, 8)
	}

	pipeline.work = pipeline.work[:1]
	pipeline.work[0] = raw

	for _, stage := range pipeline.stages {
		if stage == nil {
			return 0, ErrNilNumber
		}

		next, err := pipeline.applyStage(stage, stageOut, pipeline.work[:pipeline.workLen])

		if err != nil {
			return 0, err
		}

		stageOut = next
		pipeline.work = append(pipeline.work, stageOut)
		pipeline.workLen++
	}

	return stageOut, nil
}

func (pipeline *Pipeline) applyStage(
	stage Number, out Float64, work []Float64,
) (Float64, error) {
	stageFast, isFast := stage.(Stage)

	if isFast {
		return stageFast.Apply(out, work)
	}

	inputs := pipeline.buildStageInputs(out, work)

	return stage.Observe(inputs...), nil
}

func (pipeline *Pipeline) buildStageInputs(
	out Float64, work []Float64,
) []Number {
	pipeline.scratch = pipeline.scratch[:0]
	pipeline.scratch = append(pipeline.scratch, out)

	for _, sample := range work {
		pipeline.scratch = append(pipeline.scratch, sample)
	}

	return pipeline.scratch
}
