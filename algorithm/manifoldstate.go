package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewManifoldstate returns the manifold state equation stage.
*/
func NewManifoldstate() io.ReadWriter {
	return equation.NewManifoldstate()
}
