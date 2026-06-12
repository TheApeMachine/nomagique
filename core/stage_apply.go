package core

/*
Stage applies a pipeline step from prior output and work samples without interface fan-out.
*/
type Stage interface {
	Number
	Apply(out Float64, work []Float64) (Float64, error)
}
