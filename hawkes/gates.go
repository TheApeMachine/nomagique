package hawkes

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
)

const minFitGateHistory = 4

/*
FitGates carries series-local saturation and frenzy thresholds derived from fit history.
*/
type FitGates struct {
	SaturationRadius float64
	FrenzyAsymmetry  float64
}

/*
Ready reports whether both gates were derived from sufficient history.
*/
func (gates FitGates) Ready() bool {
	return gates.SaturationRadius > 0 && gates.FrenzyAsymmetry > 0
}

/*
FitGatesFromHistory derives saturation and frenzy gates from observed fit statistics.
*/
func FitGatesFromHistory(spectralRadii, asymmetries []float64) (FitGates, bool) {
	if len(spectralRadii) < minFitGateHistory || len(asymmetries) < minFitGateHistory {
		return FitGates{}, false
	}

	saturationArtifact := datura.Acquire("hawkes-saturation", datura.Artifact_Type_json)
	putFloat64sPayload(saturationArtifact, spectralRadii...)
	satBuf, _ := saturationArtifact.Message().Marshal()
	saturationStage := statistic.NewQuantile(0.9, stat.LinInterp, nil)
	_, _ = saturationStage.Write(satBuf)
	satOut := make([]byte, len(satBuf))
	_, _ = saturationStage.Read(satOut)
	saturationRadius, _ := readPayloadScalar(satOut)

	absAsymmetries := make([]float64, len(asymmetries))

	for index, asymmetry := range asymmetries {
		absAsymmetries[index] = math.Abs(asymmetry)
	}

	frenzyArtifact := datura.Acquire("hawkes-frenzy", datura.Artifact_Type_json)
	putFloat64sPayload(frenzyArtifact, absAsymmetries...)
	frenzyBuf, _ := frenzyArtifact.Message().Marshal()
	frenzyStage := statistic.NewQuantile(0.25, stat.LinInterp, nil)
	_, _ = frenzyStage.Write(frenzyBuf)
	frenzyOut := make([]byte, len(frenzyBuf))
	_, _ = frenzyStage.Read(frenzyOut)
	frenzyAsymmetry, _ := readPayloadScalar(frenzyOut)

	if saturationRadius <= 0 || frenzyAsymmetry <= 0 {
		return FitGates{}, false
	}

	return FitGates{
		SaturationRadius: saturationRadius,
		FrenzyAsymmetry:  frenzyAsymmetry,
	}, true
}

func putFloat64sPayload(artifact *datura.Artifact, values ...float64) {
	payload := make([]byte, 8*len(values))

	for index, value := range values {
		offset := index * 8
		binary.BigEndian.PutUint64(payload[offset:offset+8], math.Float64bits(value))
	}

	_ = artifact.SetPayload(payload)
}

func readPayloadScalar(artifactBytes []byte) (float64, bool) {
	outbound := datura.Acquire("hawkes-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(artifactBytes)
	payload, err := outbound.Payload()

	if err != nil || len(payload) != 8 {
		return 0, false
	}

	return math.Float64frombits(binary.BigEndian.Uint64(payload)), true
}
