package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewVerticality returns the pre-pump verticality equation stage.
*/
func NewVerticality() io.ReadWriter {
	return equation.NewVerticality()
}
