package algorithm

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/learning"
)

/*
Trust combines forecast scale calibration with adaptive prediction trust.
*/
type Trust struct {
	artifact   *datura.Artifact
	forecast   learning.ForecastState
	weight     learning.WeightState
	lastTrust  float64
}

/*
NewTrust creates a calibration-trust stage over predicted-vs-actual pairs.
*/
func NewTrust() *Trust {
	return &Trust{
		artifact: datura.Acquire("trust", datura.Artifact_Type_json),
	}
}

func (trust *Trust) Write(p []byte) (int, error) {
	return trust.artifact.Write(p)
}

func (trust *Trust) Read(p []byte) (int, error) {
	rehydrateArtifact(&trust.artifact, "trust", datura.Artifact_Type_json)

	payload, err := trust.artifact.Payload()

	if err == nil && len(payload) >= 16 {
		predicted := math.Float64frombits(binary.BigEndian.Uint64(payload[:8]))
		actual := math.Float64frombits(binary.BigEndian.Uint64(payload[8:16]))
		trustValue := learning.ObserveWeight(&trust.weight, predicted, actual)
		_ = learning.ObserveForecast(&trust.forecast, predicted, actual)
		trust.lastTrust = trustValue

		scale := trust.forecast.Scale
		calibration := 1 - math.Abs(1-scale)

		if calibration < 0 {
			calibration = 0
		}

		derived := trustValue * calibration
		out := make([]byte, 8)
		binary.BigEndian.PutUint64(out, math.Float64bits(derived))
		_ = trust.artifact.SetPayload(out)
	}

	return trust.artifact.Read(p)
}

func (trust *Trust) Close() error {
	return nil
}

/*
Scale returns the current forecast scale.
*/
func (trust *Trust) Scale() float64 {
	return trust.forecast.Scale
}

/*
Weight returns the current adaptive trust weight.
*/
func (trust *Trust) Weight() float64 {
	return trust.lastTrust
}

/*
Reset clears derived state.
*/
func (trust *Trust) Reset() error {
	trust.lastTrust = 0
	trust.forecast.Reset()
	trust.weight.Reset()

	return nil
}
