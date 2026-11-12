package causal

/*
Transitive causal layer from symm — containsIndex estimates do/backdoor effects on tabular desk features.
*/
func containsIndex(indices []int, nodeIndex int) bool {
	for _, skipIndex := range indices {
		if skipIndex == nodeIndex {
			return true
		}
	}

	return false
}
