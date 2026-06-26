package learning

import (
	"fmt"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
RLS is an online recursive-least-squares stage composable in nomagique.Number pipelines.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type RLS struct {
	artifact   *datura.Artifact
	stateStore *datura.Artifact
}

/*
NewRLS returns an RLS stage wired from config attributes on the artifact.
*/
func NewRLS(artifact *datura.Artifact) *RLS {
	return &RLS{
		artifact:   artifact,
		stateStore: datura.Acquire("rls-state-store", datura.APPJSON),
	}
}

func (rls *RLS) Read(payload []byte) (int, error) {
	if rls == nil || rls.artifact == nil {
		return 0, nil
	}

	state := datura.Acquire("rls-state", datura.APPJSON)

	if _, err := state.Unpack(rls.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rls: state write failed",
			err,
		))
	}

	defer state.Release()

	session, err := rls.loadSession()

	if err != nil {
		return 0, err
	}

	values := datura.Peek[[]float64](state, "batch")

	if len(values) == 0 {
		values = datura.Peek[[]float64](state, "features")
	}

	if len(values) < session.dimension+1 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rls: batch shorter than feature dimension plus target",
			nil,
		))
	}

	features := values[:session.dimension]
	target := values[len(values)-1]

	if err := session.observe(features, target); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rls: observe failed",
			err,
		))
	}

	prediction, err := session.predict(features)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rls: predict failed",
			err,
		))
	}

	session.persist(rls.stateStore, prediction)
	state.MergeOutput("value", prediction)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (rls *RLS) Write(payload []byte) (int, error) {
	rls.artifact.WithPayload(payload)

	return len(payload), nil
}

func (rls *RLS) Close() error {
	return nil
}

type rlsSession struct {
	dimension        int
	initialVariance  float64
	forgettingFactor float64
	beta             []float64
	covariance       [][]float64
}

func (rls *RLS) loadSession() (*rlsSession, error) {
	dimension := int(datura.Peek[float64](rls.artifact, "dimension"))
	initialVariance := datura.Peek[float64](rls.artifact, "initialVariance")
	forgettingFactor := datura.Peek[float64](rls.artifact, "forgettingFactor")

	if forgettingFactor == 0 {
		forgettingFactor = 1
	}

	if dimension <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"rls: dimension must be positive",
			nil,
		))
	}

	if initialVariance <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"rls: initial variance must be positive",
			nil,
		))
	}

	if forgettingFactor <= 0 || forgettingFactor > 1 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"rls: forgetting factor must be in (0,1]",
			nil,
		))
	}

	size := dimension + 1
	session := &rlsSession{
		dimension:        dimension,
		initialVariance:  initialVariance,
		forgettingFactor: forgettingFactor,
	}

	beta := datura.Peek[[]float64](rls.stateStore, "output", "beta")
	covFlat := datura.Peek[[]float64](rls.stateStore, "output", "covariance")

	if len(beta) == size && len(covFlat) == size*size {
		session.beta = append([]float64(nil), beta...)
		session.covariance = unflattenCovariance(covFlat, size)

		return session, nil
	}

	session.beta = make([]float64, size)
	session.covariance = newRLSCovariance(size, initialVariance)

	return session, nil
}

func (session *rlsSession) persist(stateStore *datura.Artifact, prediction float64) {
	stateStore.MergeOutput("value", prediction)
	stateStore.MergeOutput("beta", append([]float64(nil), session.beta...))
	stateStore.MergeOutput("covariance", flattenCovariance(session.covariance))
	stateStore.MergeOutput("covarianceDiagonal", covarianceDiagonal(session.covariance))
}

func newRLSCovariance(size int, initialVariance float64) [][]float64 {
	covariance := make([][]float64, size)

	for row := 0; row < size; row++ {
		covariance[row] = make([]float64, size)
		covariance[row][row] = initialVariance
	}

	return covariance
}

func flattenCovariance(covariance [][]float64) []float64 {
	size := len(covariance)
	flat := make([]float64, size*size)

	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			flat[row*size+col] = covariance[row][col]
		}
	}

	return flat
}

func unflattenCovariance(flat []float64, size int) [][]float64 {
	covariance := make([][]float64, size)

	for row := 0; row < size; row++ {
		covariance[row] = make([]float64, size)

		for col := 0; col < size; col++ {
			covariance[row][col] = flat[row*size+col]
		}
	}

	return covariance
}

func covarianceDiagonal(covariance [][]float64) []float64 {
	diagonal := make([]float64, len(covariance))

	for row := range covariance {
		diagonal[row] = covariance[row][row]
	}

	return diagonal
}

func (session *rlsSession) resetCovariance() {
	size := session.dimension + 1

	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			session.covariance[row][col] = 0
		}

		session.covariance[row][row] = session.initialVariance
	}
}

func (session *rlsSession) stabilizeCovariance() {
	size := len(session.covariance)
	diagonalFloor := session.initialVariance * rlsCovarianceFloorScale()

	for row := 0; row < size; row++ {
		for col := row + 1; col < size; col++ {
			averaged := (session.covariance[row][col] + session.covariance[col][row]) * 0.5
			session.covariance[row][col] = averaged
			session.covariance[col][row] = averaged
		}

		if session.covariance[row][row] < diagonalFloor {
			session.covariance[row][row] = diagonalFloor
		}
	}
}

func rlsCovarianceFloorScale() float64 {
	return math.Sqrt(math.Nextafter(1, 2) - 1)
}

func (session *rlsSession) applyForgetting() error {
	if session.forgettingFactor >= 1 {
		return nil
	}

	scale := 1 / session.forgettingFactor

	for row := range session.covariance {
		for col := range session.covariance[row] {
			session.covariance[row][col] *= scale
		}
	}

	return nil
}

func (session *rlsSession) observe(features []float64, target float64) error {
	for attempt := 0; attempt < 2; attempt++ {
		err := session.observeOnce(features, target)

		if err == nil {
			session.stabilizeCovariance()

			return nil
		}

		session.resetCovariance()

		if attempt == 1 {
			return err
		}
	}

	return fmt.Errorf("learning: rls observe failed after covariance repair")
}

func (session *rlsSession) observeOnce(features []float64, target float64) error {
	if err := session.applyForgetting(); err != nil {
		return err
	}

	if !finite(target) {
		return fmt.Errorf("learning: rls target must be finite")
	}

	if len(features) != session.dimension {
		return fmt.Errorf(
			"learning: rls expected %d features, got %d",
			session.dimension,
			len(features),
		)
	}

	design := make([]float64, session.dimension+1)
	design[0] = 1

	for index, feature := range features {
		if !finite(feature) {
			return fmt.Errorf("learning: rls feature[%d] must be finite", index)
		}

		design[index+1] = feature
	}

	size := len(design)
	px := make([]float64, size)

	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			px[row] += session.covariance[row][col] * design[col]
		}
	}

	denominator := 1.0

	for index := 0; index < size; index++ {
		denominator += design[index] * px[index]
	}

	if denominator <= 0 {
		return fmt.Errorf("learning: rls denominator must be positive")
	}

	prediction := 0.0

	for index := 0; index < size; index++ {
		prediction += session.beta[index] * design[index]
	}

	innovation := target - prediction

	for row := 0; row < size; row++ {
		gain := px[row] / denominator
		session.beta[row] += gain * innovation

		for col := 0; col < size; col++ {
			session.covariance[row][col] -= gain * px[col]
		}
	}

	return nil
}

func (session *rlsSession) predict(features []float64) (float64, error) {
	if len(features) != session.dimension {
		return 0, fmt.Errorf(
			"learning: rls expected %d features, got %d",
			session.dimension,
			len(features),
		)
	}

	forecast := session.beta[0]

	for index, feature := range features {
		if !finite(feature) {
			return 0, fmt.Errorf("learning: rls feature[%d] must be finite", index)
		}

		forecast += session.beta[index+1] * feature
	}

	if !finite(forecast) {
		return 0, fmt.Errorf("learning: rls forecast must be finite")
	}

	return forecast, nil
}
