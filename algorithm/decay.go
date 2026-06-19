package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewDecay returns the microstructure decay equation stage.
*/
func NewDecay() io.ReadWriter {
	return equation.NewDecay()
}
