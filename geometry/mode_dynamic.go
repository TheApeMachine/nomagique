package geometry

import (
	"math"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

/*
ModeDetector partitions participants into eigenmodes from origin, energy, and coupling streams.
The coupling stream is a row-major N×N affinity matrix aligned to participant order.
*/
type ModeDetector struct {
	threshold float64
	origins   core.Numbers
	energies  core.Numbers
	coupling  core.Numbers
	snap      *EigenSnap
}

/*
NewModeDetector creates an eigenmode detector over configured streams.
*/
func NewModeDetector(
	threshold float64,
	origins, energies, coupling core.Numbers,
) *ModeDetector {
	return &ModeDetector{
		threshold: threshold,
		origins:   origins,
		energies:  energies,
		coupling:  coupling,
	}
}

/*
Observe returns the dominant mode energy after partitioning participants.
*/
func (detector *ModeDetector) Observe(_ ...core.Number) core.Float64 {
	participants, couplingFn, ok := detector.participantsAndCoupling()

	if !ok {
		detector.snap = nil

		return 0
	}

	modes, dominant := DetectModes(
		participants, detector.threshold, couplingFn,
	)
	detector.snap = NewEigenSnap(modes, dominant)

	if dominant < 0 {
		return 0
	}

	return core.Float64(modes[dominant].Energy())
}

/*
ModeCount returns the number of modes from the last Observe call.
*/
func (detector *ModeDetector) ModeCount() int {
	if detector.snap == nil {
		return 0
	}

	return len(detector.snap.Modes())
}

/*
DominantEnergy returns the dominant mode energy from the last Observe call.
*/
func (detector *ModeDetector) DominantEnergy() core.Float64 {
	if detector.snap == nil {
		return 0
	}

	mode, ok := detector.snap.Dominant()

	if !ok {
		return 0
	}

	return core.Float64(mode.Energy())
}

/*
Snap returns the last eigenmode partition.
*/
func (detector *ModeDetector) Snap() *EigenSnap {
	return detector.snap
}

/*
Reset clears derived state.
*/
func (detector *ModeDetector) Reset() error {
	detector.snap = nil

	return nil
}

func (detector *ModeDetector) participantsAndCoupling() (
	[]ModeParticipant, func(uint64, uint64) float64, bool,
) {
	origins := nomagique.Samples(detector.origins)
	energies := nomagique.Samples(detector.energies)
	matrix := nomagique.Samples(detector.coupling)

	if len(origins) == 0 || len(origins) != len(energies) {
		return nil, nil, false
	}

	size := len(origins)

	if len(matrix) != size*size {
		return nil, nil, false
	}

	participants := make([]ModeParticipant, size)

	for index := range participants {
		participants[index] = ModeParticipant{
			Origin: uint64(origins[index]),
			Energy: energies[index],
		}
	}

	couplingFn := func(leftOrigin, rightOrigin uint64) float64 {
		leftIndex := originIndex(origins, leftOrigin)
		rightIndex := originIndex(origins, rightOrigin)

		if leftIndex < 0 || rightIndex < 0 {
			return 0
		}

		return matrix[leftIndex*size+rightIndex]
	}

	return participants, couplingFn, true
}

func originIndex(origins []float64, origin uint64) int {
	for index, value := range origins {
		if uint64(math.Round(value)) == origin {
			return index
		}
	}

	return -1
}
