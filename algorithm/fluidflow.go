package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewFluidflow returns the fluid-dynamics equation stage.
*/
func NewFluidflow() io.ReadWriter {
	return equation.NewFluidflow()
}
