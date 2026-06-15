package geometry

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
Eigenmode represents a cluster of field participants whose affinity
vectors are mutually coupled above a threshold. The members share
structural resonance — they respond to similar input topology. Energy
is the aggregate surprisal mass of the mode, used to rank dominance.
*/
type Eigenmode struct {
	members []uint64
	energy  float64
}

/*
Members returns the participant IDs in this mode.
*/
func (mode *Eigenmode) Members() []uint64 {
	return append([]uint64(nil), mode.members...)
}

/*
Energy returns the aggregate energy score.
*/
func (mode *Eigenmode) Energy() float64 {
	return mode.energy
}

/*
ModeParticipant carries the minimum inputs needed for eigenmode
detection: an origin ID and a scalar energy contribution.
*/
type ModeParticipant struct {
	Origin uint64
	Energy float64
}

/*
EigenSnap stores the last completed eigenmode partition.
It is swapped in by a background pass so readers can consume slightly stale
modes without blocking on ModePartition.Observe.
*/
type EigenSnap struct {
	modes       []Eigenmode
	dominantIdx int
}

/*
NewEigenSnap wraps a (modes, dominantIdx) pair into a snap so callers can hand the whole
partition around by a single pointer and swap it atomically when a new one lands.
*/
func NewEigenSnap(modes []Eigenmode, dominantIdx int) *EigenSnap {
	return &EigenSnap{
		modes:       modes,
		dominantIdx: dominantIdx,
	}
}

/*
Modes returns a defensive copy of the eigenmode partition held by this snap.
Callers cannot mutate the snap's internal state through the returned slice.
*/
func (snap *EigenSnap) Modes() []Eigenmode {
	if snap == nil {
		return nil
	}

	out := make([]Eigenmode, len(snap.modes))

	for modeIndex := range snap.modes {
		eigenmode := &snap.modes[modeIndex]
		out[modeIndex] = Eigenmode{
			members: eigenmode.Members(),
			energy:  eigenmode.Energy(),
		}
	}

	return out
}

/*
DominantIdx returns the index of the highest-energy mode, or -1 when
the partition is empty.
*/
func (snap *EigenSnap) DominantIdx() int {
	if snap == nil || len(snap.modes) == 0 {
		return -1
	}

	return snap.dominantIdx
}

/*
Dominant returns the dominant Eigenmode directly, or the zero value
when no modes were detected. The ok flag distinguishes a genuine empty
snap from a legitimate zero-energy dominant mode.
*/
func (snap *EigenSnap) Dominant() (mode Eigenmode, ok bool) {
	if snap == nil {
		return Eigenmode{}, false
	}

	if snap.dominantIdx < 0 || snap.dominantIdx >= len(snap.modes) {
		return Eigenmode{}, false
	}

	return snap.modes[snap.dominantIdx], true
}

/*
PhaseMode is the dominant finite-field phase extracted from a vector.
The lane index acts as the phase angle; amplitude and concentration
describe how collapsed the vector is around that lane.
*/
type PhaseMode struct {
	Index         int
	Amplitude     uint32
	Concentration float64
}

/*
ModePartition partitions participants into eigenmodes from origin, energy, and coupling streams.
The coupling stream is a row-major N×N affinity matrix aligned to participant order.
*/
type ModePartition[T ~float64] struct {
	threshold float64
	origins   []float64
	energies  []float64
	coupling  []float64
	snap      *EigenSnap
	output    core.Scalar[T]
}

/*
NewModePartition creates an eigenmode partition stage over configured streams.
*/
func NewModePartition[T ~float64](
	threshold float64,
	origins, energies, coupling []float64,
) *ModePartition[T] {
	return &ModePartition[T]{
		threshold: threshold,
		origins:   origins,
		energies:  energies,
		coupling:  coupling,
	}
}

/*
Observe returns the dominant mode energy after partitioning participants.
*/
func (partition *ModePartition[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	participants, couplingFn, ok := partition.participantsAndCoupling()

	if !ok {
		partition.snap = nil
		partition.output = core.Scalar[T](0)

		return partition.output
	}

	modes, dominant := partition.partition(participants, couplingFn)
	partition.snap = NewEigenSnap(modes, dominant)

	if dominant < 0 {
		partition.output = core.Scalar[T](0)

		return partition.output
	}

	partition.output = core.Scalar[T](T(modes[dominant].Energy()))

	return partition.output
}

/*
Snap returns the last eigenmode partition.
*/
func (partition *ModePartition[T]) Snap() *EigenSnap {
	return partition.snap
}

/*
Reset clears derived state.
*/
func (partition *ModePartition[T]) Reset() error {
	partition.snap = nil
	partition.output = core.Scalar[T](0)

	return nil
}

func (partition *ModePartition[T]) partition(
	participants []ModeParticipant,
	couplingFn func(leftOrigin, rightOrigin uint64) float64,
) ([]Eigenmode, int) {
	assigned := make(map[uint64]bool, len(participants))
	modes := make([]Eigenmode, 0)

	for _, participantAnchor := range participants {
		if assigned[participantAnchor.Origin] {
			continue
		}

		mode := Eigenmode{
			members: []uint64{participantAnchor.Origin},
			energy:  participantAnchor.Energy,
		}

		assigned[participantAnchor.Origin] = true

		for _, participantCandidate := range participants {
			if assigned[participantCandidate.Origin] {
				continue
			}

			if couplingFn(participantAnchor.Origin, participantCandidate.Origin) >= partition.threshold {
				mode.members = append(mode.members, participantCandidate.Origin)
				mode.energy += participantCandidate.Energy
				assigned[participantCandidate.Origin] = true
			}
		}

		modes = append(modes, mode)
	}

	if len(modes) == 0 {
		return modes, -1
	}

	dominantIdx := 0
	maxEnergy := modes[0].energy

	for modeIndex := 1; modeIndex < len(modes); modeIndex++ {
		if modes[modeIndex].energy > maxEnergy {
			maxEnergy = modes[modeIndex].energy
			dominantIdx = modeIndex
		}
	}

	return modes, dominantIdx
}

func (partition *ModePartition[T]) participantsAndCoupling() (
	[]ModeParticipant, func(uint64, uint64) float64, bool,
) {
	origins := partition.origins
	energies := partition.energies
	matrix := partition.coupling

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
			Origin: uint64(math.Round(origins[index])),
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
