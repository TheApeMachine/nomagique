package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/learning"
)

/*
NewTrust returns a calibration-trust pipeline over predicted-vs-actual pairs.
*/
func NewTrust() io.ReadWriter {
	return nomagique.Number(learning.Weight(), learning.Forecast())
}
