package correlation

/*
Multiverse estimates ensemble coupling from tiered window sets. It is an alias for
Contagion so existing call sites keep a stable name inside nomagique.Number pipelines.
*/
type Multiverse = Contagion

/*
NewMultiverse wires window sets into a coupling estimator.
*/
func NewMultiverse(
	windowSets []*WindowSet,
	tiers TierWindows,
	config ContagionConfig,
) *Multiverse {
	return NewContagion(windowSets, tiers, config)
}
