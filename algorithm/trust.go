package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/learning"
)

/*
NewTrust returns a calibration-trust pipeline over predicted-vs-actual pairs.
*/
func NewTrust() io.ReadWriteCloser {
	return nomagique.Number(
		learning.NewTrustWeight(datura.Acquire("trust-weight-config", datura.APPJSON).WithAttributes(datura.Map[any]{
			"sampleKey": "sample",
			"pairedKey": "paired",
		})),
		learning.Forecast(datura.Acquire("forecast-config", datura.APPJSON).WithAttributes(datura.Map[any]{
			"sampleKey": "predicted",
			"pairedKey": "actual",
		})),
	)
}
