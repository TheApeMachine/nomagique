package logic

import "github.com/theapemachine/datura"

/*
Condition evaluates whether a circuit rule should fire.
*/
type Condition interface {
	Match(artifact *datura.Artifact) bool
	ResetOperands()
}
