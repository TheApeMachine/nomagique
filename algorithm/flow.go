package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewFlow returns the cumulative volume delta equation stage.
*/
func NewFlow() io.ReadWriter {
	return equation.NewFlow()
}
