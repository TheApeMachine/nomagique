package equation

import "fmt"

/*
CancelFillRatio returns cancel volume divided by matched fill volume.
*/
func CancelFillRatio(cancel, fill float64) (float64, error) {
	if cancel <= 0 || fill <= 0 {
		return 0, fmt.Errorf("equation: cancel and fill must be positive")
	}

	return cancel / fill, nil
}
