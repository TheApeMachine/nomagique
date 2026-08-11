package fluid

import (
	"fmt"
	"math"
)

// SettlementStatus holds the convergence state of each coupled physics subsystem.
type SettlementStatus struct {
	WaveSettled       bool `json:"waveSettled"`
	FluidSettled      bool `json:"fluidSettled"`
	GuidanceSettled   bool `json:"guidanceSettled"`
	PopulationSettled bool `json:"populationSettled"`
	IsFullySettled    bool `json:"isFullySettled"`

	RelativeWaveChange float32 `json:"relativeWaveChange"`
	PressureGradDelta  float64 `json:"pressureGradDelta"`
	GuidanceRMSDelta   float32 `json:"guidanceRmsDelta"`
	ParticleCount      int     `json:"particleCount"`
}

type SettlementTracker struct {
	Tolerance          float32
	MinStableSteps     int
	consecutiveSettled int

	prevPressureGrad  float64
	prevGuidanceRMS   float32
	prevParticleCount int
	initialized       bool
}

func NewSettlementTracker(tolerance float32, minStableSteps int) *SettlementTracker {
	return &SettlementTracker{
		Tolerance:      tolerance,
		MinStableSteps: minStableSteps,
	}
}

// Check evaluates whether the resident Metal domain has reached equilibrium.
func (st *SettlementTracker) Check(domain *Domain, diag Diagnostics) (SettlementStatus, error) {
	reading, err := domain.Reading()
	if err != nil {
		return SettlementStatus{}, fmt.Errorf("settlement check failed: %w", err)
	}

	currentParticles := domain.ParticleCount()
	status := SettlementStatus{ParticleCount: currentParticles}

	if !st.initialized {
		st.prevPressureGrad = reading.PressureGradNorm
		st.prevGuidanceRMS = diag.GuidanceRMS
		st.prevParticleCount = currentParticles
		st.initialized = true
		return status, nil
	}

	// 1. Integrator Stability Check
	if diag.Halvings > 0 {
		st.consecutiveSettled = 0
		return status, nil
	}

	// 2. Quantum Wave Settlement (PsiDeltaRMS / PsiRMS)
	if diag.PsiRMS > 0 {
		status.RelativeWaveChange = diag.PsiDeltaRMS / diag.PsiRMS
		status.WaveSettled = status.RelativeWaveChange <= st.Tolerance
	} else {
		status.WaveSettled = true
	}

	// 3. Fluid Hydrostatic Settlement (|d(PressureGradNorm)/dt|)
	status.PressureGradDelta = math.Abs(reading.PressureGradNorm - st.prevPressureGrad)
	status.FluidSettled = float32(status.PressureGradDelta) <= st.Tolerance

	// 4. Pilot-Wave Guidance Settlement (|d(GuidanceRMS)/dt|)
	status.GuidanceRMSDelta = float32(math.Abs(float64(diag.GuidanceRMS - st.prevGuidanceRMS)))
	status.GuidanceSettled = status.GuidanceRMSDelta <= st.Tolerance

	// 5. Inelastic Merge Settlement (Particle count no longer decreasing)
	status.PopulationSettled = (currentParticles == st.prevParticleCount)

	// Combine all 4 subsystems
	allSubsystemsSettled := status.WaveSettled &&
		status.FluidSettled &&
		status.GuidanceSettled &&
		status.PopulationSettled

	if allSubsystemsSettled {
		st.consecutiveSettled++
	} else {
		st.consecutiveSettled = 0
	}

	status.IsFullySettled = st.consecutiveSettled >= st.MinStableSteps

	// Update historical state for next step
	st.prevPressureGrad = reading.PressureGradNorm
	st.prevGuidanceRMS = diag.GuidanceRMS
	st.prevParticleCount = currentParticles

	return status, nil
}
