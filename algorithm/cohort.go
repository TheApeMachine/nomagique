package algorithm

import (
	"io"

	"github.com/theapemachine/nomagique/equation"
)

/*
NewCohort returns the peer cohort equation stage.
*/
func NewCohort() io.ReadWriter {
	return equation.NewCohort()
}
