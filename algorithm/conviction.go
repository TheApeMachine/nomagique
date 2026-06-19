package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewConviction returns the market-breadth conviction equation stage.
*/
func NewConviction() io.ReadWriter {
	return equation.NewConviction()
}
