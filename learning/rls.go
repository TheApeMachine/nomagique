package learning

import (
	"fmt"
	"math"
)

/*
RLSConfig configures recursive least squares.
*/
type RLSConfig struct {
	Dimension        int
	InitialVariance  float64
	ForgettingFactor float64
}

/*
RLSSample carries one feature vector and target.
*/
type RLSSample struct {
	Features []float64
	Target   float64
}

/*
RLSOutput reports the latest prediction and retained linear state.
*/
type RLSOutput struct {
	Value              float64
	Beta               []float64
	Covariance         []float64
	CovarianceDiagonal []float64
}

/*
RLS is an online recursive-least-squares learner.
*/
type RLS struct {
	config  RLSConfig
	session *rlsSession
}

/*
NewRLS returns a typed RLS learner.
*/
func NewRLS(config RLSConfig) (*RLS, error) {
	learner := &RLS{
		config: config,
	}

	session, err := learner.loadSession()

	if err != nil {
		return nil, err
	}

	learner.session = session

	return learner, nil
}

/*
Measure observes one sample and returns the current prediction.
*/
func (rls *RLS) Measure(sample RLSSample) (RLSOutput, error) {
	if rls == nil || rls.session == nil {
		return RLSOutput{}, fmt.Errorf("learning: rls session required")
	}

	if err := rls.session.observe(sample.Features, sample.Target); err != nil {
		return RLSOutput{}, fmt.Errorf("learning: rls observe failed: %w", err)
	}

	prediction, err := rls.session.predict(sample.Features)

	if err != nil {
		return RLSOutput{}, fmt.Errorf("learning: rls predict failed: %w", err)
	}

	return RLSOutput{
		Value:              prediction,
		Beta:               append([]float64(nil), rls.session.beta...),
		Covariance:         flattenCovariance(rls.session.covariance),
		CovarianceDiagonal: covarianceDiagonal(rls.session.covariance),
	}, nil
}

/*
Predict evaluates features against the retained coefficients without observing a
target. This keeps a live forecast strictly prior to the outcome used to train
the next step.
*/
func (rls *RLS) Predict(features []float64) (RLSOutput, error) {
	if rls == nil || rls.session == nil {
		return RLSOutput{}, fmt.Errorf("learning: rls session required")
	}

	prediction, err := rls.session.predict(features)

	if err != nil {
		return RLSOutput{}, fmt.Errorf("learning: rls predict failed: %w", err)
	}

	return RLSOutput{
		Value:              prediction,
		Beta:               append([]float64(nil), rls.session.beta...),
		Covariance:         flattenCovariance(rls.session.covariance),
		CovarianceDiagonal: covarianceDiagonal(rls.session.covariance),
	}, nil
}

type rlsSession struct {
	dimension        int
	initialVariance  float64
	forgettingFactor float64
	beta             []float64
	covariance       [][]float64
}

func (rls *RLS) loadSession() (*rlsSession, error) {
	config := rls.config

	if config.ForgettingFactor == 0 {
		config.ForgettingFactor = 1
	}

	if config.Dimension <= 0 {
		return nil, fmt.Errorf("learning: rls dimension must be positive")
	}

	if config.InitialVariance <= 0 {
		return nil, fmt.Errorf("learning: rls initial variance must be positive")
	}

	if config.ForgettingFactor <= 0 || config.ForgettingFactor > 1 {
		return nil, fmt.Errorf("learning: rls forgetting factor must be in (0,1]")
	}

	size := config.Dimension + 1

	return &rlsSession{
		dimension:        config.Dimension,
		initialVariance:  config.InitialVariance,
		forgettingFactor: config.ForgettingFactor,
		beta:             make([]float64, size),
		covariance:       newRLSCovariance(size, config.InitialVariance),
	}, nil
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
