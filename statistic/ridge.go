package statistic

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
)

/*
RidgeSolver solves regularized normal equations with Gonum LAPACK backends.

Use when a cross-section or feature panel is rank-deficient: ridge strength is
derived from the condition number rather than a fixed lambda.
*/
type RidgeSolver struct{}

/*
NewRidgeSolver creates a ridge solver.
*/
func NewRidgeSolver() *RidgeSolver {
	return &RidgeSolver{}
}

/*
Solve returns the ridge-regularized solution to normal·x = vector.
*/
func (solver *RidgeSolver) Solve(normal [][]float64, vector []float64) ([]float64, error) {
	size := len(vector)

	if size == 0 || len(normal) != size {
		return nil, fmt.Errorf("statistic: ridge system size mismatch")
	}

	for row := 0; row < size; row++ {
		if len(normal[row]) != size {
			return nil, fmt.Errorf("statistic: ridge normal matrix is not square")
		}
	}

	lambda := solver.ridgeLambda(normal)
	symmetric := mat.NewSymDense(size, nil)

	for row := 0; row < size; row++ {
		for col := row; col < size; col++ {
			value := normal[row][col]

			if row == col && row > 0 {
				value += lambda
			}

			symmetric.SetSym(row, col, value)
		}
	}

	rightHand := mat.NewVecDense(size, vector)

	var cholesky mat.Cholesky

	if cholesky.Factorize(symmetric) {
		var solution mat.VecDense

		if err := cholesky.SolveVecTo(&solution, rightHand); err != nil {
			return nil, fmt.Errorf("statistic: cholesky solve: %w", err)
		}

		out := make([]float64, size)

		for index := range out {
			out[index] = solution.AtVec(index)
		}

		return out, nil
	}

	dense := mat.NewDense(size, size, flattenRegularizedSquare(normal, lambda))
	var qr mat.QR

	qr.Factorize(dense)

	var solution mat.VecDense

	if err := qr.SolveVecTo(&solution, false, rightHand); err != nil {
		return nil, fmt.Errorf("statistic: qr solve: %w", err)
	}

	out := make([]float64, size)

	for index := range out {
		out[index] = solution.AtVec(index)
	}

	return out, nil
}

func flattenRegularizedSquare(matrix [][]float64, lambda float64) []float64 {
	size := len(matrix)
	flat := make([]float64, size*size)

	for row := 0; row < size; row++ {
		copy(flat[row*size:(row+1)*size], matrix[row])

		if row > 0 {
			flat[row*size+row] += lambda
		}
	}

	return flat
}

/*
ridgeLambda derives the regularization strength from the matrix's own
conditioning, not a fixed constant. A well-conditioned system needs only a
machine-epsilon nudge for numerical stability; a rank-deficient one (a constant
or collinear predictor — e.g. a thin pair whose order flow never varies) needs
real damping so the normal matrix becomes positive-definite and the solve returns
a coefficient shrunk toward zero rather than failing. Zero shrinkage IS the
correct structural answer for a predictor that carries no resolvable signal.

lambda scales with the diagonal mean (so it is unit-consistent with the matrix)
and with how far the estimated condition number exceeds the numerically safe
limit: at or below the limit it is the epsilon floor; far beyond it (or infinite,
the singular case) it rises to a fraction of the diagonal mean — enough to
guarantee Cholesky succeeds on the regularized system.
*/
func (solver *RidgeSolver) ridgeLambda(normal [][]float64) float64 {
	trace := 0.0
	size := float64(len(normal))

	for row := 0; row < len(normal); row++ {
		trace += normal[row][row]
	}

	if trace <= 0 || size <= 0 {
		return 0
	}

	base := trace / size
	machineEpsilon := machineSqrtEpsilon()
	floor := base * machineEpsilon

	condition := solver.conditionEstimate(normal)
	conditionLimit := 1 / machineEpsilon

	// Well-conditioned: only the numerical floor.
	if condition <= conditionLimit && !math.IsInf(condition, 0) {
		return floor
	}

	// Rank-deficient or singular: damp by a fraction of the diagonal scale.
	// The fraction grows with the excess condition number and saturates at the
	// diagonal mean so a perfectly singular column (condition = +Inf) gets the
	// full base damping that makes the system positive-definite.
	if math.IsInf(condition, 0) {
		return floor + base
	}

	excess := condition / conditionLimit
	fraction := 1 - 1/excess

	return floor + base*fraction
}

func machineSqrtEpsilon() float64 {
	return math.Sqrt(math.Nextafter(1, 2) - 1)
}

func (solver *RidgeSolver) conditionEstimate(normal [][]float64) float64 {
	size := len(normal)

	if size == 0 {
		return 0
	}

	diagonals := make([]float64, size)

	for row := 0; row < size; row++ {
		if len(normal[row]) != size {
			return math.Inf(1)
		}

		diagonals[row] = normal[row][row]

		if diagonals[row] <= 0 {
			return math.Inf(1)
		}
	}

	maxEigenBound := 0.0
	minEigenBound := math.Inf(1)
	pivotFloor := machineSqrtEpsilon()

	for row := 0; row < size; row++ {
		radius := 0.0

		for col := 0; col < size; col++ {
			if col == row {
				continue
			}

			normalizer := math.Sqrt(diagonals[row] * diagonals[col])

			if normalizer <= 0 {
				return math.Inf(1)
			}

			radius += math.Abs(normal[row][col]) / normalizer
		}

		upper := 1 + radius
		lower := 1 - radius

		if upper > maxEigenBound {
			maxEigenBound = upper
		}

		if lower < minEigenBound {
			minEigenBound = lower
		}
	}

	if minEigenBound <= pivotFloor {
		return math.Inf(1)
	}

	return maxEigenBound / minEigenBound
}
