package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
StdDev computes the sample standard deviation over retained history.
*/
type StdDev struct {
	artifact *datura.Artifact
}

/*
NewStdDev creates a standard-deviation stage.
*/
func NewStdDev() *StdDev {
	return &StdDev{
		artifact: datura.Acquire("stddev", datura.APPJSON),
	}
}

func (stdDev *StdDev) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](stdDev.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return stdDev.artifact.Read(p)
	}

	history := datura.Peek[[]float64](stdDev.artifact, "history")
	history = append(history, sample)
	stdDev.artifact.Poke(history, "history")

	value := 0.0

	if len(history) >= 2 {
		value = stat.StdDev(history, nil)
	}

	stdDev.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return stdDev.artifact.Read(p)
}

func (stdDev *StdDev) Write(p []byte) (int, error) {
	return stdDev.artifact.Write(p)
}

func (stdDev *StdDev) Close() error {
	return nil
}
