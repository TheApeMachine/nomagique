package nomagique

import "io"

/*
Number composes the stages into one pipeline and is named "no magic number" on
purpose: connect it as close to the source as possible so there is never a
temptation to insert configuration layers full of static/magic values between
the data and the algorithm. Write an artifact into the head, read the result
out of the tail; the stages in between carry everything.
*/
func Number(stages ...io.ReadWriter) io.ReadWriter {
	return &pipeline{stages: stages}
}

type pipeline struct {
	stages []io.ReadWriter
}

/*
Write feeds the artifact into the head stage.
*/
func (pipeline *pipeline) Write(p []byte) (int, error) {
	if len(pipeline.stages) == 0 {
		return len(p), nil
	}

	return pipeline.stages[0].Write(p)
}

/*
Read runs every stage in order, copying each stage's output into the next, and
returns the tail stage's result.
*/
func (pipeline *pipeline) Read(p []byte) (int, error) {
	if len(pipeline.stages) == 0 {
		return 0, io.EOF
	}

	for index := 0; index < len(pipeline.stages)-1; index++ {
		if _, err := io.Copy(pipeline.stages[index+1], pipeline.stages[index]); err != nil && err != io.EOF {
			return 0, err
		}
	}

	return pipeline.stages[len(pipeline.stages)-1].Read(p)
}

func (pipeline *pipeline) Close() error {
	return nil
}
