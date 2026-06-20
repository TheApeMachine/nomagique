package nomagique

import (
	"io"

	"github.com/theapemachine/datura/transport"
)

/*
Number composes the stages into one pipeline and is named "no magic number" on
purpose: connect it as close to the source as possible so there is never a
temptation to insert configuration layers full of static/magic values between
the data and the algorithm. Write an artifact into the head, read the result
out of the tail; the stages in between carry everything.
*/
func Number(stages ...io.ReadWriteCloser) io.ReadWriteCloser {
	return transport.NewPipeline(stages...)
}
