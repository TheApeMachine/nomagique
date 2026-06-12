package core

import "sync"

var pipelinePool sync.Pool

/*
AcquirePipeline returns a pooled pipeline wired to stages.
*/
func AcquirePipeline(stages []Number) *Pipeline {
	pipeline, isPipeline := pipelinePool.Get().(*Pipeline)

	if !isPipeline {
		return NewPipeline(stages)
	}

	pipeline.Bind(stages)

	return pipeline
}

/*
ReleasePipeline returns a pipeline to the pool.
*/
func ReleasePipeline(pipeline *Pipeline) {
	pipelinePool.Put(pipeline)
}
