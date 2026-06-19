package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewDepth returns the liquidity depth equation stage.
*/
func NewDepth() io.ReadWriter {
	return equation.NewDepth()
}
