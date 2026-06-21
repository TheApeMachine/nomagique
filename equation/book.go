package equation

/*
CancelFillRatio returns cancel volume divided by matched fill volume.
*/
func CancelFillRatio(cancel, fill float64) float64 {
	if cancel <= 0 || fill <= 0 {
		return 0
	}

	return cancel / fill
}
