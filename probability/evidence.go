package probability

import "math"

/*
EvidenceGeomean combines positive evidence margins into one score in (0, 1).
*/
func EvidenceGeomean(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}

	product := 1.0

	for _, value := range values {
		if value <= 0 {
			return 0
		}

		product *= value
	}

	return math.Pow(product, 1/float64(len(values)))
}
