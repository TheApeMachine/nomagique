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
}

/*
NewIntervalCoupling creates an interval-correlation dynamic over two series.
*/
func NewIntervalCoupling(left, right *IntervalSeries) *IntervalCoupling {
	return &IntervalCoupling{
		artifact: datura.Acquire("interval-coupling", datura.APPJSON).RetainStageAttributes(),
		left:     left,
		right:    right,
	}
}

func (coupling *IntervalCoupling) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](coupling.artifact, "output") == nil

	coupling.artifact.Clear("sample")
	coupling.artifact.Clear("paired")

	n, err := coupling.artifact.Write(p)

	if bootstrap {
		coupling.artifact.Clear("output")
	}

	return n, err
}

func (coupling *IntervalCoupling) Read(p []byte) (int, error) {
	if coupling == nil {
		return 0, nil
	}

	value, ok := IntervalCorrelation(coupling.left, coupling.right)

	if !ok {
		coupling.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return coupling.artifact.Read(p)
	}

	coupling.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return coupling.artifact.Read(p)
}

func (coupling *IntervalCoupling) Close() error {
	return nil
}

func (coupling *IntervalCoupling) Reset() error {
	coupling.artifact.Clear("output")

	return nil
}
