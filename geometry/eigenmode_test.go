package geometry

import "testing"

func TestEigenmodeMembers(t *testing.T) {
	mode := &Eigenmode{
		members: []uint64{1, 2, 3},
		energy:  4.5,
	}

	copySlice := mode.Members()
	copySlice[0] = 99

	if mode.members[0] != 1 {
		t.Fatalf("members were not copied")
	}

	if len(copySlice) != 3 {
		t.Fatalf("copy length = %d, want 3", len(copySlice))
	}
}

func TestEigenmodeEnergy(t *testing.T) {
	mode := &Eigenmode{energy: 12.25}

	if mode.Energy() != 12.25 {
		t.Fatalf("energy = %f, want 12.25", mode.Energy())
	}
}

func TestModePartitionDirect(t *testing.T) {
	partition := &ModePartition{
		threshold: 1,
	}
	participants := []ModeParticipant{
		{Origin: 10, Energy: 1},
		{Origin: 20, Energy: 2},
		{Origin: 30, Energy: 4},
	}
	coupling := func(leftOrigin uint64, rightOrigin uint64) float64 {
		if leftOrigin == rightOrigin {
			return 1
		}

		if leftOrigin == 10 && rightOrigin == 20 {
			return 1
		}

		if leftOrigin == 20 && rightOrigin == 10 {
			return 1
		}

		return 0
	}

	modes, dominant := partition.partition(participants, coupling)
	if dominant != 1 {
		t.Fatalf("dominant index = %d, want 1", dominant)
	}

	if len(modes) != 2 {
		t.Fatalf("modes = %d, want 2", len(modes))
	}
}
