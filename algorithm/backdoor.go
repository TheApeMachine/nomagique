package algorithm

import "github.com/theapemachine/nomagique/causal"

/*
NewBackdoor returns a typed backdoor estimator over tabular rows.
*/
func NewBackdoor(config causal.BackdoorConfig) *causal.Backdoor {
	return causal.NewBackdoor(config)
}
