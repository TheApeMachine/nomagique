package probability

import (
	"fmt"
	"math"
)

/*
EvidenceGeomean combines positive evidence margins into one score in (0, 1).
*/
func EvidenceGeomean(values ...float64) (float64, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("probability: evidence geomean requires at least one value")
	}

	product := 1.0

	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, fmt.Errorf("probability: evidence geomean value[%d] is non-finite", index)
		}

		if value <= 0 {
			return 0, fmt.Errorf("probability: evidence geomean value[%d] must be positive", index)
		}

		product *= value
	}

	return math.Pow(product, 1/float64(len(values))), nil
}
