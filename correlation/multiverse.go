package correlation

/*
Multiverse estimates ensemble coupling from tiered window sets. It is an alias for
Contagion so existing call sites keep a stable name inside nomagique.Number pipelines.
*/
type Multiverse[T ~float64] = Contagion[T]

/*
NewMultiverse wires window sets into a coupling estimator.
*/
func NewMultiverse[T ~float64](
	windowSets []*WindowSet[T],
	tiers TierWindows,
	config ContagionConfig,
) *Multiverse[T] {
	return NewContagion[T](windowSets, tiers, config)
}
