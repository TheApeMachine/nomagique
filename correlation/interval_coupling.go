package correlation

import (
	"github.com/theapemachine/datura"
)

/*
IntervalCoupling tracks Hayashi-Yoshida correlation between two interval accumulators.
*/
type IntervalCoupling struct {
	artifact *datura.Artifact
	left     *IntervalSeries
	right    *IntervalSeries
	output   float64
}

/*
NewIntervalCoupling creates an interval-correlation dynamic over two series.
*/
func NewIntervalCoupling(left, right *IntervalSeries) *IntervalCoupling {
	return &IntervalCoupling{
		artifact: datura.Acquire("interval-coupling", datura.Artifact_Type_json),
		left:     left,
		right:    right,
	}
}

func (coupling *IntervalCoupling) Write(p []byte) (int, error) {
	return coupling.artifact.Write(p)
}

func (coupling *IntervalCoupling) Read(p []byte) (int, error) {
	if coupling == nil {
		return coupling.artifact.Read(p)
	}

	value, ok := IntervalCorrelation(coupling.left, coupling.right)

	if ok {
		coupling.output = value
		putFloat64Payload(&coupling.artifact, "interval-coupling", coupling.output)
	}

	return coupling.artifact.Read(p)
}

func (coupling *IntervalCoupling) Close() error {
	return nil
}

func (coupling *IntervalCoupling) Reset() error {
	coupling.output = 0

	return nil
}
