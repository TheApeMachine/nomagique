//go:build darwin && cgo

//go:generate go run ../manifold/metallibgen

package fluid

/*
#cgo CFLAGS: -fobjc-arc -I${SRCDIR}
#cgo CXXFLAGS: -x objective-c++ -std=c++17 -fobjc-arc -I${SRCDIR}
#cgo LDFLAGS: -framework Metal -framework Foundation -framework CoreFoundation -framework Accelerate
#include "bridge.h"
*/
import "C"

import (
	_ "embed"
	"fmt"
	"runtime"
	"unsafe"
)

const (
	sharedDisplayWidth  = 64
	sharedDisplayHeight = 64
)

//go:embed kernels.metallib
var fluidMetallib []byte

type domainHandle unsafe.Pointer

/*
NewDomain loads the generated Sensorium Metal library and creates one resident
coupled domain. Loading is deliberately fail-fast; there is no CPU backend.
*/
func NewDomain(config Config) (*Domain, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Sensorium Domain.base medium: dry air γ=1.4, M=0.02897, L=1m → ω-natural G.
	si := DefaultSIConstants()
	units, err := OmegaNaturalUnitSystem(1.4, 0.02897, 1, si)

	if err != nil {
		return nil, err
	}

	physical, err := ConstantsFromSI(units, si)

	if err != nil {
		return nil, err
	}

	bridgeConfig := C.FluidConfig{
		grid_x:    C.uint32_t(config.Grid.X),
		grid_y:    C.uint32_t(config.Grid.Y),
		grid_z:    C.uint32_t(config.Grid.Z),
		spacing:   C.float(config.Grid.Spacing),
		max_delta: C.float(config.MaxDelta),
		omega_min: C.float(config.OmegaMin),
		omega_max: C.float(config.OmegaMax),
		gravity_g: C.float(physical.G),
	}
	errorBuffer := make([]byte, 4096)
	handle := C.fluid_domain_new(
		&bridgeConfig,
		unsafe.Pointer(&fluidMetallib[0]),
		C.size_t(len(fluidMetallib)),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)
	runtime.KeepAlive(fluidMetallib)

	if handle == nil {
		return nil, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return &Domain{
		handle: domainHandle(handle),
		config: config,
	}, nil
}

/*
Step is the legacy one-shot path: replace the resident population with the
supplied batch, advance once (including inelastic merge), and write evolved
state back into particles. Merge may compact the live prefix; callers that
re-step must reslice to ParticleCount(). Streaming callers should use
Append + Advance with real content IDs instead.
*/
func (domain *Domain) Step(particles []Particle) (Diagnostics, error) {
	if domain == nil || domain.handle == nil {
		return Diagnostics{}, fmt.Errorf("fluid: domain is closed")
	}

	if err := validateParticles(particles, domain.config); err != nil {
		return Diagnostics{}, err
	}

	bridgeParticles := bridgeFromParticles(particles)
	var bridgeDiagnostics C.FluidDiagnostics
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_step(
		unsafe.Pointer(domain.handle),
		&bridgeParticles[0],
		C.uint32_t(len(bridgeParticles)),
		&bridgeDiagnostics,
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return Diagnostics{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	copyBridgeToParticles(bridgeParticles, particles)
	return diagnosticsFromBridge(bridgeDiagnostics), nil
}

/*
Append packs one batch into Shared staging and blits it into Private resident
Metal storage without advancing physics. contentIDs are Sensorium content
identities used for inelastic merge (universal content_token_ids). Evolved
history grows by GPU blit, not host re-upload. Returns the starting index of
the appended range.
*/
func (domain *Domain) Append(particles []Particle, contentIDs []uint32) (int, error) {
	if domain == nil || domain.handle == nil {
		return 0, fmt.Errorf("fluid: domain is closed")
	}

	if err := validateParticles(particles, domain.config); err != nil {
		return 0, err
	}

	if len(contentIDs) != len(particles) {
		return 0, fmt.Errorf("fluid: content ID count must match particle count")
	}

	bridgeParticles := bridgeFromParticles(particles)
	var start C.uint32_t
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_append(
		unsafe.Pointer(domain.handle),
		&bridgeParticles[0],
		(*C.uint32_t)(unsafe.Pointer(&contentIDs[0])),
		C.uint32_t(len(bridgeParticles)),
		&start,
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return 0, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return int(start), nil
}

/*
Advance runs the Sensorium manifold physics graph without re-uploading history:
thermo.step → omegawave.step → quantum_flow.step (project / smooth / pilot).
*/
func (domain *Domain) Advance() (Diagnostics, error) {
	if domain == nil || domain.handle == nil {
		return Diagnostics{}, fmt.Errorf("fluid: domain is closed")
	}

	var bridgeDiagnostics C.FluidDiagnostics
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_advance(
		unsafe.Pointer(domain.handle),
		&bridgeDiagnostics,
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return Diagnostics{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return diagnosticsFromBridge(bridgeDiagnostics), nil
}

/*
ParticleCount returns the resident population size in Metal storage.
*/
func (domain *Domain) ParticleCount() int {
	if domain == nil || domain.handle == nil {
		return 0
	}

	return int(C.fluid_domain_particle_count(unsafe.Pointer(domain.handle)))
}

/*
Retain keeps only the listed resident indices, preserving content IDs so later
Append/Advance merge identities remain valid. Indices must be unique and in
range of the current ParticleCount.
*/
func (domain *Domain) Retain(indices []uint32) error {
	if domain == nil || domain.handle == nil {
		return fmt.Errorf("fluid: domain is closed")
	}

	if len(indices) == 0 {
		errorBuffer := make([]byte, 4096)
		result := C.fluid_domain_retain(
			unsafe.Pointer(domain.handle),
			nil,
			0,
			(*C.char)(unsafe.Pointer(&errorBuffer[0])),
			C.int(len(errorBuffer)),
		)

		if result == 0 {
			return fmt.Errorf("fluid: %s", cString(errorBuffer))
		}

		return nil
	}

	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_retain(
		unsafe.Pointer(domain.handle),
		(*C.uint32_t)(unsafe.Pointer(&indices[0])),
		C.uint32_t(len(indices)),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return nil
}

/*
ReadParticles copies one resident range out of Metal storage after advance.
*/
func (domain *Domain) ReadParticles(start, count int) ([]Particle, error) {
	if domain == nil || domain.handle == nil {
		return nil, fmt.Errorf("fluid: domain is closed")
	}

	if start < 0 || count < 0 {
		return nil, fmt.Errorf("fluid: particle read range is invalid")
	}

	if count == 0 {
		return nil, nil
	}

	bridgeParticles := make([]C.FluidParticle, count)
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read_particles(
		unsafe.Pointer(domain.handle),
		&bridgeParticles[0],
		C.uint32_t(start),
		C.uint32_t(count),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return nil, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	particles := make([]Particle, count)
	copyBridgeToParticles(bridgeParticles, particles)
	return particles, nil
}

/*
ReadSpatialIDs copies post-merge Morton spatial token IDs for one resident range.
Each ID is (cell_morton << 8) | (content & 0xFF), matching Sensorium thermo.step.
*/
func (domain *Domain) ReadSpatialIDs(start, count int) ([]uint32, error) {
	if domain == nil || domain.handle == nil {
		return nil, fmt.Errorf("fluid: domain is closed")
	}

	if start < 0 || count < 0 {
		return nil, fmt.Errorf("fluid: spatial ID read range is invalid")
	}

	if count == 0 {
		return nil, nil
	}

	ids := make([]uint32, count)
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read_spatial_ids(
		unsafe.Pointer(domain.handle),
		(*C.uint32_t)(unsafe.Pointer(&ids[0])),
		C.uint32_t(start),
		C.uint32_t(count),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return nil, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return ids, nil
}

func bridgeFromParticles(particles []Particle) []C.FluidParticle {
	bridgeParticles := make([]C.FluidParticle, len(particles))

	for index, particle := range particles {
		bridgeParticles[index] = C.FluidParticle{
			position_x: C.float(particle.Position.X),
			position_y: C.float(particle.Position.Y),
			position_z: C.float(particle.Position.Z),
			velocity_x: C.float(particle.Velocity.X),
			velocity_y: C.float(particle.Velocity.Y),
			velocity_z: C.float(particle.Velocity.Z),
			mass:       C.float(particle.Mass),
			heat:       C.float(particle.Heat),
			energy:     C.float(particle.Energy),
			phase:      C.float(particle.Phase),
			omega:      C.float(particle.Omega),
		}
	}

	return bridgeParticles
}

func copyBridgeToParticles(bridgeParticles []C.FluidParticle, particles []Particle) {
	for index, particle := range bridgeParticles {
		particles[index] = Particle{
			Position: Vector{
				X: float32(particle.position_x),
				Y: float32(particle.position_y),
				Z: float32(particle.position_z),
			},
			Velocity: Vector{
				X: float32(particle.velocity_x),
				Y: float32(particle.velocity_y),
				Z: float32(particle.velocity_z),
			},
			Mass:   float32(particle.mass),
			Heat:   float32(particle.heat),
			Energy: float32(particle.energy),
			Phase:  float32(particle.phase),
			Omega:  float32(particle.omega),
		}
	}
}

func diagnosticsFromBridge(bridgeDiagnostics C.FluidDiagnostics) Diagnostics {
	return Diagnostics{
		CFLRate:      float32(bridgeDiagnostics.cfl_rate),
		DeltaAdv:     float32(bridgeDiagnostics.delta_adv),
		DeltaDiffuse: float32(bridgeDiagnostics.delta_diffuse),
		DeltaDerived: float32(bridgeDiagnostics.delta_derived),
		DeltaUsed:    float32(bridgeDiagnostics.delta_used),
		Halvings:     uint32(bridgeDiagnostics.halvings),
		PsiRMS:       float32(bridgeDiagnostics.psi_rms),
		PsiDeltaRMS:  float32(bridgeDiagnostics.psi_delta_rms),
		GuidanceRMS:  float32(bridgeDiagnostics.guidance_rms),
	}
}

/*
Wave reads the resident omega lattice without advancing it.
*/
func (domain *Domain) Wave() ([]WaveMode, error) {
	if domain == nil || domain.handle == nil {
		return nil, fmt.Errorf("fluid: domain is closed")
	}

	modeCount := uint32(C.fluid_domain_mode_count(unsafe.Pointer(domain.handle)))
	modes := make([]C.FluidWaveMode, modeCount)
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read_wave(
		unsafe.Pointer(domain.handle),
		&modes[0],
		C.uint32_t(modeCount),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return nil, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	wave := make([]WaveMode, modeCount)

	for index, mode := range modes {
		wave[index] = WaveMode{
			Omega:     float32(mode.omega),
			Real:      float32(mode.real),
			Imaginary: float32(mode.imaginary),
			Linewidth: float32(mode.linewidth),
		}
	}

	return wave, nil
}

/*
Reading reads the post-step gas and wave observables without advancing the
resident domain.
*/
func (domain *Domain) Reading() (Reading, error) {
	if domain == nil || domain.handle == nil {
		return Reading{}, fmt.Errorf("fluid: domain is closed")
	}

	var bridgeReading C.FluidReading
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read(
		unsafe.Pointer(domain.handle),
		&bridgeReading,
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return Reading{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return Reading{
		PressureGradX:    float64(bridgeReading.pressure_grad_x),
		PressureGradY:    float64(bridgeReading.pressure_grad_y),
		PressureGradZ:    float64(bridgeReading.pressure_grad_z),
		PressureGradNorm: float64(bridgeReading.pressure_grad_norm),
		Divergence:       float64(bridgeReading.divergence),
		CoherenceMag2:    float64(bridgeReading.coherence_mag2),
		GuidanceSpeed:    float64(bridgeReading.guidance_speed),
		ViscosityProxy:   float64(bridgeReading.viscosity_proxy),
	}, nil
}

/*
Projection reads the X-Z maximum projection used for field inspection without
advancing the resident domain.
*/
func (domain *Domain) Projection() (Projection, error) {
	if domain == nil || domain.handle == nil {
		return Projection{}, fmt.Errorf("fluid: domain is closed")
	}

	length := domain.config.Grid.X * domain.config.Grid.Z
	projection := Projection{
		Grid:      domain.config.Grid,
		Density:   make([]float32, length),
		Coherence: make([]float32, length),
		GuidanceX: make([]float32, length),
		GuidanceZ: make([]float32, length),
	}
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read_projection(
		unsafe.Pointer(domain.handle),
		(*C.float)(unsafe.Pointer(&projection.Density[0])),
		(*C.float)(unsafe.Pointer(&projection.Coherence[0])),
		(*C.float)(unsafe.Pointer(&projection.GuidanceX[0])),
		(*C.float)(unsafe.Pointer(&projection.GuidanceZ[0])),
		C.uint32_t(length),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)
	runtime.KeepAlive(projection)

	if result == 0 {
		return Projection{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return projection, nil
}

/*
Display runs the Metal project/colormap/splat pass and returns one Shared RGBA8
frame plus occupancy stats. Particles stay Private on GPU; only the picture is
read back.
*/
func (domain *Domain) Display() ([]byte, DisplayStats, error) {
	if domain == nil || domain.handle == nil {
		return nil, DisplayStats{}, fmt.Errorf("fluid: domain is closed")
	}

	width := uint32(sharedDisplayWidth)
	height := uint32(sharedDisplayHeight)
	rgba := make([]byte, width*height*4)
	var stats C.FluidDisplayStats
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read_display(
		unsafe.Pointer(domain.handle),
		(*C.uint8_t)(unsafe.Pointer(&rgba[0])),
		C.uint32_t(len(rgba)),
		&stats,
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)
	runtime.KeepAlive(rgba)

	if result == 0 {
		return nil, DisplayStats{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return rgba, DisplayStats{
		Width:            uint32(stats.width),
		Height:           uint32(stats.height),
		RhoOccupied:      uint32(stats.rho_occupied),
		PsiOccupied:      uint32(stats.psi_occupied),
		RhoMax:           float32(stats.rho_max),
		PsiMax:           float32(stats.psi_max),
		GuidanceOccupied: uint32(stats.guidance_occupied),
		GuidanceMax:      float32(stats.guidance_max),
	}, nil
}

/*
Close releases the resident Metal library, pipelines, and buffers.
*/
func (domain *Domain) Close() error {
	if domain == nil || domain.handle == nil {
		return nil
	}

	C.fluid_domain_free(unsafe.Pointer(domain.handle))
	domain.handle = nil
	return nil
}

func cString(buffer []byte) string {
	for index, value := range buffer {
		if value == 0 {
			return string(buffer[:index])
		}
	}

	return string(buffer)
}
