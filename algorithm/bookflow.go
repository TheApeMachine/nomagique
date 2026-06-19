package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewBookflow returns the directional book-flow equation stage.
*/
func NewBookflow() io.ReadWriter {
	return equation.NewBookflow()
}
