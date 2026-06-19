package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewBookQuality returns the book-flow quality equation stage.
*/
func NewBookQuality() io.ReadWriter {
	return equation.NewBookQuality()
}
